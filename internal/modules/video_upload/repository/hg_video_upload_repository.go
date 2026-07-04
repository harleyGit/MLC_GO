package VideoUploadRepositoryPackage

import (
	"MLC_GO/internal/events"
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	"MLC_GO/internal/outbox"
	hg_time "MLC_GO/internal/pkg/hg_time"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Repository 封装视频投稿模块的 MySQL 访问。
// 当前设计不使用数据库外键，所有关联通过 user_id/submission_id/video_id 字段和索引维护。
type Repository struct {
	db *sql.DB
}

// UploadedVideo 是上传单个视频文件后写库所需的内部数据结构。
// 它不是 HTTP DTO，避免把存储字段和前端协议强绑定。
type UploadedVideo struct {
	// SubmissionID 稿件业务 ID。
	SubmissionID string
	// VideoID 视频业务 ID。
	VideoID string
	// UserID 当前登录用户业务 ID。
	UserID string
	// PartNumber 分 P 序号，从 1 开始。
	PartNumber uint32
	// Title 默认视频标题，通常由原始文件名去扩展名得到。
	Title string
	// FileName 原始文件名。
	FileName string
	// FilePath 服务端保存后的访问路径。
	FilePath string
	// FileSize 文件大小，单位字节。
	FileSize int64
	// MimeType 上传请求声明的 MIME 类型。
	MimeType string
	// MD5 文件内容摘要。
	MD5 string
}

// NewRepository 创建视频投稿仓储。
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateUploadedVideo 创建或更新稿件的上传记录。
// 第一个分 P 会创建 video_submissions；后续分 P 复用 submission_id 并累加数量和大小。
func (r *Repository) CreateUploadedVideo(ctx context.Context, video UploadedVideo) error {
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.InsertOrUpdateVideoSubmissionSQL, video.SubmissionID, video.UserID, video.Title, video.FileSize)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, SQLQueriesPackage.InsertOrUpdateVideoFileSQL, video.VideoID, video.SubmissionID, video.UserID, video.PartNumber, video.Title, video.FileName, video.FilePath, video.FileSize, video.MimeType, video.MD5)

	return err
}

// SaveSubmission 保存完整稿件配置。
// 因为本模块不使用外键，所有写入都带 userID/submissionID 限定，避免误更新其他用户数据。
func (r *Repository) SaveSubmission(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest) error {
	return r.SaveSubmissionWithEvents(ctx, userID, req)
}

// SaveSubmissionWithEvents 在同一个 MySQL 事务中保存稿件配置和 Outbox 事件。
// 初学者重点：不要 Save() 后立刻 Producer.Send()，否则会出现“数据库成功但 Kafka 失败”的不一致。
func (r *Repository) SaveSubmissionWithEvents(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest, domainEvents ...events.DomainEvent) error {
	// 开启本地事务，把稿件主表、分 P 配置、标签、定时发布、商业推广和 Outbox 事件作为一个原子单元提交。
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// Commit 成功后 Rollback 会返回 sql.ErrTxDone；defer 保证异常路径释放事务资源。
	defer tx.Rollback()

	if err := r.saveSubmissionTx(ctx, tx, userID, req); err != nil {
		return err
	}
	if len(domainEvents) > 0 {
		// Outbox writer 复用同一个 tx，避免数据库写入成功但事件记录丢失。
		writer := outbox.NewRepository(r.db, "mlc.domain.events")
		for _, event := range domainEvents {
			if event == nil {
				continue
			}
			if err := writer.SaveTx(ctx, tx, event); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (r *Repository) saveSubmissionTx(ctx context.Context, tx *sql.Tx, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest) error {
	// CardConfig 是动态卡片配置，使用 JSON 存储；空配置转 NULL，避免无意义空对象占用字段。
	cardConfig, err := json.Marshal(req.CardConfig)
	if err != nil {
		return err
	}
	if len(req.CardConfig) == 0 {
		cardConfig = nil
	}

	videoCount, totalSize, err := r.getSubmissionTotalsTx(ctx, tx, req.SubmissionID, userID)
	if err != nil {
		return err
	}

	// 主表保存稿件级配置，WHERE/唯一键都带 submissionID + userID，防止跨用户误写。
	_, err = tx.ExecContext(ctx, SQLQueriesPackage.SaveSubmissionSQL, req.SubmissionID, userID, req.Title, req.CoverURL, req.Category, req.VideoType, req.SourceURL, req.Description,
		boolToInt(req.AllowSecondaryCreation), boolToInt(req.Watermark), req.Visibility, req.Declaration, nullableJSON(cardConfig),
		boolToInt(req.DolbyAudio), boolToInt(req.HiresAudio), boolToInt(req.CloseDanmaku), boolToInt(req.CloseComment),
		boolToInt(req.FeaturedComment), req.DynamicDescription, boolToInt(req.HideFromProfile), videoCount, totalSize, req.Status,
		submitTime(req.Status))
	if err != nil {
		return err
	}

	for _, video := range req.Videos {
		// 分 P 配置和标签在同一个事务内更新，保证列表展示不会看到半更新状态。
		if err = r.updateVideoConfigTx(ctx, tx, userID, req.SubmissionID, video); err != nil {
			return err
		}
		if err = r.replaceTagsTx(ctx, tx, video.VideoID, video.Tags); err != nil {
			return err
		}
	}

	if err = r.saveScheduleTx(ctx, tx, userID, req.SubmissionID, req.Schedule); err != nil {
		return err
	}

	return r.saveCommercialTx(ctx, tx, userID, req.SubmissionID, req.Commercial)
}

// getSubmissionTotals 根据已上传的视频文件重新计算稿件总数和总大小。
// 这样前端不需要可信地上报 video_count/total_size，避免被篡改。
func (r *Repository) getSubmissionTotals(ctx context.Context, submissionID, userID string) (int, int64, error) {
	return r.getSubmissionTotalsTx(ctx, r.db, submissionID, userID)
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (r *Repository) getSubmissionTotalsTx(ctx context.Context, q queryRower, submissionID, userID string) (int, int64, error) {
	var videoCount int
	var totalSize int64
	// 重新按已上传文件计算总数和总大小，防止客户端篡改提交请求中的聚合字段。
	err := q.QueryRowContext(ctx, SQLQueriesPackage.GetSubmissionTotalsSQL, submissionID, userID).Scan(&videoCount, &totalSize)
	return videoCount, totalSize, err
}

// updateVideoConfig 更新单个分 P 的表单配置。
func (r *Repository) updateVideoConfig(ctx context.Context, userID string, submissionID string, video VideoUploadDtoPackage.VideoConfigRequest) error {
	return r.updateVideoConfigTx(ctx, r.db, userID, submissionID, video)
}

func (r *Repository) updateVideoConfigTx(ctx context.Context, execer sqlExecer, userID string, submissionID string, video VideoUploadDtoPackage.VideoConfigRequest) error {
	_, err := execer.ExecContext(ctx, SQLQueriesPackage.UpdateVideoFileConfigSQL, video.PartNumber, video.Title, video.CoverURL, video.VideoType, video.SourceURL, video.Category, video.Description, video.VideoID, submissionID, userID)
	return err
}

// replaceTags 使用先删后插保存标签。
// 标签最多 7 个，数据量很小；这种写法比逐项 diff 更简单且行为稳定。
func (r *Repository) replaceTags(ctx context.Context, videoID string, tags []string) error {
	return r.replaceTagsTx(ctx, r.db, videoID, tags)
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (r *Repository) replaceTagsTx(ctx context.Context, execer sqlExecer, videoID string, tags []string) error {
	// 先清理旧标签再插入新标签，保证删除标签的场景也能正确落库。
	if _, err := execer.ExecContext(ctx, SQLQueriesPackage.DeleteVideoTagsByVideoIDSQL, videoID); err != nil {
		return err
	}
	for _, tag := range tags {
		if _, err := execer.ExecContext(ctx, SQLQueriesPackage.InsertVideoTagSQL, videoID, tag); err != nil {
			return err
		}
	}
	return nil
}

// saveSchedule 保存或删除稿件级定时发布配置。
func (r *Repository) saveSchedule(ctx context.Context, userID string, submissionID string, schedule *VideoUploadDtoPackage.ScheduleRequest) error {
	return r.saveScheduleTx(ctx, r.db, userID, submissionID, schedule)
}

func (r *Repository) saveScheduleTx(ctx context.Context, execer sqlExecer, userID string, submissionID string, schedule *VideoUploadDtoPackage.ScheduleRequest) error {
	if schedule == nil || !schedule.Enabled || schedule.ScheduledTime == nil {
		// 前端关闭定时发布时删除旧配置，避免历史定时任务继续生效。
		_, err := execer.ExecContext(ctx, SQLQueriesPackage.DeleteScheduledPublishSQL, submissionID, userID)
		return err
	}

	// 客户端时间统一解析为服务端可比较的 time.Time，避免直接信任字符串格式。
	scheduledTime, err := hg_time.ParseClientTime(*schedule.ScheduledTime)
	if err != nil {
		return err
	}

	_, err = execer.ExecContext(ctx, SQLQueriesPackage.InsertOrUpdateScheduledPublishSQL, submissionID, userID, scheduledTime)
	return err
}

// saveCommercial 保存或删除稿件级商业推广配置。
func (r *Repository) saveCommercial(ctx context.Context, userID string, submissionID string, commercial *VideoUploadDtoPackage.CommercialRequest) error {
	return r.saveCommercialTx(ctx, r.db, userID, submissionID, commercial)
}

func (r *Repository) saveCommercialTx(ctx context.Context, execer sqlExecer, userID string, submissionID string, commercial *VideoUploadDtoPackage.CommercialRequest) error {
	if commercial == nil || !commercial.Enabled {
		// 关闭商业推广时删除旧配置，保持请求语义为最终态覆盖。
		_, err := execer.ExecContext(ctx, SQLQueriesPackage.DeleteCommercialPromotionSQL, submissionID, userID)
		return err
	}

	_, err := execer.ExecContext(ctx, SQLQueriesPackage.InsertOrUpdateCommercialPromotionSQL, submissionID, userID, commercial.PromotionType, commercial.PromotionName, commercial.PromotionForm)
	return err
}

// boolToInt 把 Go bool 映射到 MySQL TINYINT(1)。
func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// nullableJSON 把空 JSON 配置转成 SQL NULL，避免数据库里存无意义的 "{}"。
func nullableJSON(value []byte) interface{} {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

// submitTime 仅提交审核时写 submit_time；保存草稿时保持 NULL。
func submitTime(status string) interface{} {
	if status == "reviewing" {
		return time.Now().UTC()
	}
	return nil
}

// GetVideoList 获取已提交审核的视频列表（offset 分页，兼容旧调用）。
func (r *Repository) GetVideoList(ctx context.Context, page, pageSize int) ([]VideoUploadDtoPackage.VideoListItem, int, error) {
	total, err := r.GetVideoListTotal(ctx)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []VideoUploadDtoPackage.VideoListItem{}, 0, nil
	}
	offset := (page - 1) * pageSize
	videos, err := r.queryVideoList(ctx, SQLQueriesPackage.GetVideoListSQL, pageSize, offset)
	return videos, total, err
}

// GetVideoListTotal 获取已提交审核的视频总数。
func (r *Repository) GetVideoListTotal(ctx context.Context) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.GetVideoListTotalSQL).Scan(&total)
	return total, err
}

// GetVideoStatusCounts 按状态精确回源列表计数器。
// 仅用于 Redis 计数器初始化或补偿，常规高并发查询应走 Redis Hash，避免 COUNT(*) 成为热点。
func (r *Repository) GetVideoStatusCounts(ctx context.Context) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.GetVideoStatusCountsSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counters := map[string]int64{
		"reviewing": 0,
		"published": 0,
	}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counters[status] = count
	}
	return counters, rows.Err()
}

// GetSubmissionStatus 点查单个稿件状态，用于 Redis 计数器写侧 delta 计算。
func (r *Repository) GetSubmissionStatus(ctx context.Context, submissionID string, userID string) (string, bool, error) {
	var status string
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.GetVideoSubmissionStatusSQL, submissionID, userID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return status, true, nil
}

// GetVideoListByCursor 使用游标分页获取视频列表，避免亿级数据量下 OFFSET 深分页扫描。
// cursor 格式为 "submitTime|submissionID"，首次调用传空字符串。
// 多查一条用于判断是否还有下一页。
func (r *Repository) GetVideoListByCursor(ctx context.Context, cursor string, limit int) ([]VideoUploadDtoPackage.VideoListItem, error) {
	if cursor == "" {
		return r.queryVideoList(ctx, SQLQueriesPackage.GetVideoListByCursorFirstSQL, limit)
	}

	parts := strings.SplitN(cursor, "|", 2)
	if len(parts) != 2 {
		return r.queryVideoList(ctx, SQLQueriesPackage.GetVideoListByCursorFirstSQL, limit)
	}

	return r.queryVideoList(ctx, SQLQueriesPackage.GetVideoListByCursorSQL, parts[0], parts[0], parts[1], limit)
}

// queryVideoList 执行视频列表查询并扫描结果，消除重复代码。
func (r *Repository) queryVideoList(ctx context.Context, query string, args ...interface{}) ([]VideoUploadDtoPackage.VideoListItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	videos := make([]VideoUploadDtoPackage.VideoListItem, 0, 20)
	for rows.Next() {
		var item VideoUploadDtoPackage.VideoListItem
		var submitTime, createdAt sql.NullTime
		var coverURL, videoID, filePath, fileName, mimeType sql.NullString
		var fileSize sql.NullInt64
		var partNumber sql.NullInt32

		if err := rows.Scan(
			&item.SubmissionID,
			&item.UserID,
			&item.Title,
			&coverURL,
			&item.Category,
			&item.VideoType,
			&item.Description,
			&item.Visibility,
			&item.Status,
			&item.VideoCount,
			&item.TotalSize,
			&submitTime,
			&createdAt,
			&videoID,
			&filePath,
			&fileName,
			&fileSize,
			&mimeType,
			&partNumber,
		); err != nil {
			return nil, err
		}

		if coverURL.Valid {
			item.CoverURL = coverURL.String
		}
		if videoID.Valid {
			item.VideoID = videoID.String
		}
		if filePath.Valid {
			item.FilePath = filePath.String
		}
		if fileName.Valid {
			item.FileName = fileName.String
		}
		if fileSize.Valid {
			item.FileSize = fileSize.Int64
		}
		if mimeType.Valid {
			item.MimeType = mimeType.String
		}
		if partNumber.Valid {
			item.PartNumber = uint32(partNumber.Int32)
		}
		if submitTime.Valid {
			item.SubmitTime = submitTime.Time.Format(time.RFC3339)
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}

		videos = append(videos, item)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return videos, nil
}

// EnsureVideoListIndex 确保视频列表查询所需的索引存在。
// 在服务启动时调用，创建 (status, submit_time) 联合索引以优化查询性能。
func (r *Repository) EnsureVideoListIndex(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.CreateVideoSubmissionStatusTimeIndexSQL)
	return err
}

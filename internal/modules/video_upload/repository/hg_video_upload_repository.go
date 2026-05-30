package VideoUploadRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/infrastructure/persistence/mysql/queries"
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	hg_time "MLC_GO/internal/pkg/hg_time"
	"context"
	"database/sql"
	"encoding/json"
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
	cardConfig, err := json.Marshal(req.CardConfig)
	if err != nil {
		return err
	}
	if len(req.CardConfig) == 0 {
		cardConfig = nil
	}

	videoCount, totalSize, err := r.getSubmissionTotals(ctx, req.SubmissionID, userID)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, SQLQueriesPackage.SaveSubmissionSQL, req.SubmissionID, userID, req.Title, req.CoverURL, req.Category, req.VideoType, req.SourceURL, req.Description,
		boolToInt(req.AllowSecondaryCreation), boolToInt(req.Watermark), req.Visibility, req.Declaration, nullableJSON(cardConfig),
		boolToInt(req.DolbyAudio), boolToInt(req.HiresAudio), boolToInt(req.CloseDanmaku), boolToInt(req.CloseComment),
		boolToInt(req.FeaturedComment), req.DynamicDescription, boolToInt(req.HideFromProfile), videoCount, totalSize, req.Status,
		submitTime(req.Status))
	if err != nil {
		return err
	}

	for _, video := range req.Videos {
		if err = r.updateVideoConfig(ctx, userID, req.SubmissionID, video); err != nil {
			return err
		}
		if err = r.replaceTags(ctx, video.VideoID, video.Tags); err != nil {
			return err
		}
	}

	if err = r.saveSchedule(ctx, userID, req.SubmissionID, req.Schedule); err != nil {
		return err
	}

	return r.saveCommercial(ctx, userID, req.SubmissionID, req.Commercial)
}

// getSubmissionTotals 根据已上传的视频文件重新计算稿件总数和总大小。
// 这样前端不需要可信地上报 video_count/total_size，避免被篡改。
func (r *Repository) getSubmissionTotals(ctx context.Context, submissionID, userID string) (int, int64, error) {
	var videoCount int
	var totalSize int64
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.GetSubmissionTotalsSQL, submissionID, userID).Scan(&videoCount, &totalSize)
	return videoCount, totalSize, err
}

// updateVideoConfig 更新单个分 P 的表单配置。
func (r *Repository) updateVideoConfig(ctx context.Context, userID string, submissionID string, video VideoUploadDtoPackage.VideoConfigRequest) error {
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.UpdateVideoFileConfigSQL, video.PartNumber, video.Title, video.CoverURL, video.VideoType, video.SourceURL, video.Category, video.Description, video.VideoID, submissionID, userID)
	return err
}

// replaceTags 使用先删后插保存标签。
// 标签最多 7 个，数据量很小；这种写法比逐项 diff 更简单且行为稳定。
func (r *Repository) replaceTags(ctx context.Context, videoID string, tags []string) error {
	if _, err := r.db.ExecContext(ctx, SQLQueriesPackage.DeleteVideoTagsByVideoIDSQL, videoID); err != nil {
		return err
	}
	for _, tag := range tags {
		if _, err := r.db.ExecContext(ctx, SQLQueriesPackage.InsertVideoTagSQL, videoID, tag); err != nil {
			return err
		}
	}
	return nil
}

// saveSchedule 保存或删除稿件级定时发布配置。
func (r *Repository) saveSchedule(ctx context.Context, userID string, submissionID string, schedule *VideoUploadDtoPackage.ScheduleRequest) error {
	if schedule == nil || !schedule.Enabled || schedule.ScheduledTime == nil {
		_, err := r.db.ExecContext(ctx, SQLQueriesPackage.DeleteScheduledPublishSQL, submissionID, userID)
		return err
	}

	scheduledTime, err := hg_time.ParseClientTime(*schedule.ScheduledTime)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, SQLQueriesPackage.InsertOrUpdateScheduledPublishSQL, submissionID, userID, scheduledTime)
	return err
}

// saveCommercial 保存或删除稿件级商业推广配置。
func (r *Repository) saveCommercial(ctx context.Context, userID string, submissionID string, commercial *VideoUploadDtoPackage.CommercialRequest) error {
	if commercial == nil || !commercial.Enabled {
		_, err := r.db.ExecContext(ctx, SQLQueriesPackage.DeleteCommercialPromotionSQL, submissionID, userID)
		return err
	}

	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.InsertOrUpdateCommercialPromotionSQL, submissionID, userID, commercial.PromotionType, commercial.PromotionName, commercial.PromotionForm)
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

	return r.queryVideoList(ctx, SQLQueriesPackage.GetVideoListByCursorSQL, limit, parts[0], parts[1])
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

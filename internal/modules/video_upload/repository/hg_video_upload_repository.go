package VideoUploadRepositoryPackage

import (
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	"context"
	"database/sql"
	"encoding/json"
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
	_, err := r.db.ExecContext(ctx, `
INSERT INTO video_submissions (
    submission_id, user_id, title, video_count, total_size, status
) VALUES (?, ?, ?, 1, ?, 'draft')
ON DUPLICATE KEY UPDATE
    video_count = video_count + 1,
    total_size = total_size + VALUES(total_size),
    updated_at = CURRENT_TIMESTAMP
`, video.SubmissionID, video.UserID, video.Title, video.FileSize)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO video_files (
    video_id, submission_id, user_id, part_number, title, file_name, file_path,
    file_size, mime_type, md5, upload_status, upload_progress, transcode_status
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'completed', 100.00, 'pending')
ON DUPLICATE KEY UPDATE
    title = VALUES(title),
    file_name = VALUES(file_name),
    file_path = VALUES(file_path),
    file_size = VALUES(file_size),
    mime_type = VALUES(mime_type),
    md5 = VALUES(md5),
    upload_status = 'completed',
    upload_progress = 100.00,
    updated_at = CURRENT_TIMESTAMP
`, video.VideoID, video.SubmissionID, video.UserID, video.PartNumber, video.Title, video.FileName, video.FilePath, video.FileSize, video.MimeType, video.MD5)

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

	_, err = r.db.ExecContext(ctx, `
INSERT INTO video_submissions (
    submission_id, user_id, title, cover_url, category, video_type, source_url, description,
    allow_secondary_creation, watermark, visibility, declaration, card_config, dolby_audio,
    hires_audio, close_danmaku, close_comment, featured_comment, dynamic_description,
    hide_from_profile, video_count, total_size, status, submit_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    title = VALUES(title),
    cover_url = VALUES(cover_url),
    category = VALUES(category),
    video_type = VALUES(video_type),
    source_url = VALUES(source_url),
    description = VALUES(description),
    allow_secondary_creation = VALUES(allow_secondary_creation),
    watermark = VALUES(watermark),
    visibility = VALUES(visibility),
    declaration = VALUES(declaration),
    card_config = VALUES(card_config),
    dolby_audio = VALUES(dolby_audio),
    hires_audio = VALUES(hires_audio),
    close_danmaku = VALUES(close_danmaku),
    close_comment = VALUES(close_comment),
    featured_comment = VALUES(featured_comment),
    dynamic_description = VALUES(dynamic_description),
    hide_from_profile = VALUES(hide_from_profile),
    video_count = VALUES(video_count),
    total_size = VALUES(total_size),
    status = VALUES(status),
    submit_time = VALUES(submit_time),
    updated_at = CURRENT_TIMESTAMP
`, req.SubmissionID, userID, req.Title, req.CoverURL, req.Category, req.VideoType, req.SourceURL, req.Description,
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
	err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(file_size), 0)
FROM video_files
WHERE submission_id = ? AND user_id = ?
`, submissionID, userID).Scan(&videoCount, &totalSize)
	return videoCount, totalSize, err
}

// updateVideoConfig 更新单个分 P 的表单配置。
func (r *Repository) updateVideoConfig(ctx context.Context, userID string, submissionID string, video VideoUploadDtoPackage.VideoConfigRequest) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE video_files
SET part_number = ?, title = ?, cover_url = ?, video_type = ?, source_url = ?, category = ?, description = ?, updated_at = CURRENT_TIMESTAMP
WHERE video_id = ? AND submission_id = ? AND user_id = ?
`, video.PartNumber, video.Title, video.CoverURL, video.VideoType, video.SourceURL, video.Category, video.Description, video.VideoID, submissionID, userID)
	return err
}

// replaceTags 使用先删后插保存标签。
// 标签最多 7 个，数据量很小；这种写法比逐项 diff 更简单且行为稳定。
func (r *Repository) replaceTags(ctx context.Context, videoID string, tags []string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM video_tags WHERE video_id = ?`, videoID); err != nil {
		return err
	}
	for _, tag := range tags {
		if _, err := r.db.ExecContext(ctx, `INSERT INTO video_tags (video_id, tag_name) VALUES (?, ?)`, videoID, tag); err != nil {
			return err
		}
	}
	return nil
}

// saveSchedule 保存或删除稿件级定时发布配置。
func (r *Repository) saveSchedule(ctx context.Context, userID string, submissionID string, schedule *VideoUploadDtoPackage.ScheduleRequest) error {
	if schedule == nil || !schedule.Enabled {
		_, err := r.db.ExecContext(ctx, `DELETE FROM video_scheduled_publish WHERE submission_id = ? AND user_id = ?`, submissionID, userID)
		return err
	}

	scheduledTime, err := parseClientTime(schedule.ScheduledTime)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO video_scheduled_publish (submission_id, user_id, scheduled_time, status)
VALUES (?, ?, ?, 'pending')
ON DUPLICATE KEY UPDATE
    scheduled_time = VALUES(scheduled_time),
    status = 'pending',
    updated_at = CURRENT_TIMESTAMP
`, submissionID, userID, scheduledTime)
	return err
}

// saveCommercial 保存或删除稿件级商业推广配置。
func (r *Repository) saveCommercial(ctx context.Context, userID string, submissionID string, commercial *VideoUploadDtoPackage.CommercialRequest) error {
	if commercial == nil || !commercial.Enabled {
		_, err := r.db.ExecContext(ctx, `DELETE FROM video_commercial_promotion WHERE submission_id = ? AND user_id = ?`, submissionID, userID)
		return err
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO video_commercial_promotion (submission_id, user_id, promotion_type, promotion_name, promotion_form)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    promotion_type = VALUES(promotion_type),
    promotion_name = VALUES(promotion_name),
    promotion_form = VALUES(promotion_form),
    updated_at = CURRENT_TIMESTAMP
`, submissionID, userID, commercial.PromotionType, commercial.PromotionName, commercial.PromotionForm)
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

// parseClientTime 兼容浏览器 datetime-local 和 RFC3339 两种时间格式。
func parseClientTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	return time.ParseInLocation("2006-01-02T15:04", value, time.Local)
}

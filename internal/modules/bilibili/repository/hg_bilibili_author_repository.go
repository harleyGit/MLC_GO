package BilibiliRepositoryPackage

import (
	BilibiliDtoPackage "MLC_GO/internal/modules/bilibili/dto"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository 封装 Bilibili 作者空间的只读 MySQL 查询。
type Repository struct{ db *sql.DB }

// NewRepository 创建作者空间仓储。
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// GetProfile 按业务用户 ID 读取公开资料。
func (r *Repository) GetProfile(ctx context.Context, userID string) (BilibiliDtoPackage.HGAuthorProfileResponse, error) {
	var profile BilibiliDtoPackage.HGAuthorProfileResponse
	var nickname string
	var createdAt time.Time
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectBilibiliAuthorProfileSQL, userID).Scan(
		&profile.UserID, &profile.UserName, &nickname, &profile.Signature, &profile.Gender, &profile.AvatarURL, &createdAt,
	)
	if err != nil {
		return profile, fmt.Errorf("query bilibili author profile: %w", err)
	}
	profile.DisplayName = nickname
	if profile.DisplayName == "" {
		profile.DisplayName = profile.UserName
	}
	if profile.DisplayName == "" {
		profile.DisplayName = profile.UserID
	}
	profile.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return profile, nil
}

// GetVideos 按发布时间复合游标读取公开且未从空间隐藏的投稿。
func (r *Repository) GetVideos(ctx context.Context, userID string, cursorTime *time.Time, cursorID string, limit int) ([]BilibiliDtoPackage.HGAuthorVideoItem, error) {
	var rows *sql.Rows
	var err error
	if cursorTime == nil {
		rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.SelectBilibiliAuthorVideosFirstSQL, userID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, SQLQueriesPackage.SelectBilibiliAuthorVideosByCursorSQL, userID, *cursorTime, *cursorTime, cursorID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("query bilibili author videos: %w", err)
	}
	defer rows.Close()

	videos := make([]BilibiliDtoPackage.HGAuthorVideoItem, 0, limit)
	for rows.Next() {
		var item BilibiliDtoPackage.HGAuthorVideoItem
		var publishTime time.Time
		if err := rows.Scan(&item.SubmissionID, &item.VideoID, &item.UserID, &item.Title, &item.CoverURL, &item.Category,
			&item.Description, &item.Duration, &item.FilePath, &publishTime); err != nil {
			return nil, fmt.Errorf("scan bilibili author video: %w", err)
		}
		item.PublishTime = publishTime.UTC().Format(time.RFC3339Nano)
		videos = append(videos, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bilibili author videos: %w", err)
	}
	return videos, nil
}

// CountVideos 统计作者可在空间展示的已发布视频数。
func (r *Repository) CountVideos(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.CountBilibiliAuthorVideosSQL, userID).Scan(&count)
	return count, err
}

// CountFollowing 统计作者当前有效关注数。
func (r *Repository) CountFollowing(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.CountBilibiliAuthorFollowingSQL, userID).Scan(&count)
	return count, err
}

// SumFollowers 从固定分片统计表有界聚合粉丝数。
func (r *Repository) SumFollowers(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.SumBilibiliAuthorFollowersSQL, userID).Scan(&count)
	return count, err
}

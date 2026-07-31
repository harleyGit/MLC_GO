package VideoInteractionRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"fmt"
	"time"
)

// HGProjectionCursor 是四条投影流共享的稳定复合 keyset 游标。
// 状态表使用 UpdatedAt+RowID，分片统计使用 UpdatedAt+EntityID+ShardID 处理同时间戳并列行。
type HGProjectionCursor struct {
	UpdatedAt time.Time `json:"updated_at"`
	RowID     uint64    `json:"row_id,omitempty"`
	EntityID  string    `json:"entity_id,omitempty"`
	ShardID   uint16    `json:"shard_id,omitempty"`
}

// HGVideoStateProjection 表示一个用户对一个视频的 MySQL 绝对互动状态。
type HGVideoStateProjection struct {
	UserID          string
	SubmissionID    string
	InteractionType string
	Active          bool
	Quantity        int64
	Cursor          HGProjectionCursor
}

// HGFollowStateProjection 表示一个关注关系的 MySQL 绝对状态，inactive 也必须投影为 0 以清理旧值。
type HGFollowStateProjection struct {
	FollowerID string
	FolloweeID string
	Active     bool
	Cursor     HGProjectionCursor
}

// HGVideoCountProjection 是一个视频全部 64 个统计 shard 的绝对聚合值。
type HGVideoCountProjection struct {
	SubmissionID  string
	LikeCount     int64
	CoinCount     int64
	FavoriteCount int64
	ShareCount    int64
	Cursor        HGProjectionCursor
}

// HGFollowCountProjection 是一个用户全部粉丝统计 shard 的绝对聚合值。
type HGFollowCountProjection struct {
	UserID        string
	FollowerCount int64
	Cursor        HGProjectionCursor
}

// ListVideoStates 使用 migration 14 索引读取一页状态变更，不使用 OFFSET 或无界结果集。
func (r *Repository) ListVideoStates(ctx context.Context, cursor HGProjectionCursor, cutoff time.Time, limit int) ([]HGVideoStateProjection, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectVideoStateProjectionPageSQL, cutoff, cursor.UpdatedAt, cursor.UpdatedAt, cursor.RowID, limit)
	if err != nil {
		return nil, fmt.Errorf("query video state projection page: %w", err)
	}
	defer rows.Close()
	result := make([]HGVideoStateProjection, 0, limit)
	for rows.Next() {
		var item HGVideoStateProjection
		if err := rows.Scan(&item.Cursor.RowID, &item.UserID, &item.SubmissionID, &item.InteractionType, &item.Active, &item.Quantity, &item.Cursor.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan video state projection: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate video state projection: %w", err)
	}
	return result, nil
}

// ListFollowStates 使用 updated_at+id 复合游标读取一页关注变更。
func (r *Repository) ListFollowStates(ctx context.Context, cursor HGProjectionCursor, cutoff time.Time, limit int) ([]HGFollowStateProjection, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectFollowStateProjectionPageSQL, cutoff, cursor.UpdatedAt, cursor.UpdatedAt, cursor.RowID, limit)
	if err != nil {
		return nil, fmt.Errorf("query follow state projection page: %w", err)
	}
	defer rows.Close()
	result := make([]HGFollowStateProjection, 0, limit)
	for rows.Next() {
		var item HGFollowStateProjection
		if err := rows.Scan(&item.Cursor.RowID, &item.FollowerID, &item.FolloweeID, &item.Active, &item.Cursor.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan follow state projection: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate follow state projection: %w", err)
	}
	return result, nil
}

// ListVideoCounts 先按 keyset 发现变更 shard，再仅聚合该页涉及的视频，避免全表 GROUP BY。
func (r *Repository) ListVideoCounts(ctx context.Context, cursor HGProjectionCursor, cutoff time.Time, limit int) ([]HGVideoCountProjection, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectVideoCountProjectionPageSQL,
		cutoff, cursor.UpdatedAt, cursor.UpdatedAt, cursor.EntityID, cursor.UpdatedAt, cursor.EntityID, cursor.ShardID, limit)
	if err != nil {
		return nil, fmt.Errorf("query video count projection page: %w", err)
	}
	defer rows.Close()
	result := make([]HGVideoCountProjection, 0, limit)
	for rows.Next() {
		var item HGVideoCountProjection
		if err := rows.Scan(&item.Cursor.UpdatedAt, &item.SubmissionID, &item.Cursor.ShardID, &item.LikeCount, &item.CoinCount, &item.FavoriteCount, &item.ShareCount); err != nil {
			return nil, fmt.Errorf("scan video count projection: %w", err)
		}
		item.Cursor.EntityID = item.SubmissionID
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate video count projection: %w", err)
	}
	return result, nil
}

// ListFollowCounts 先按 keyset 发现变更 shard，再仅聚合该页涉及的用户。
func (r *Repository) ListFollowCounts(ctx context.Context, cursor HGProjectionCursor, cutoff time.Time, limit int) ([]HGFollowCountProjection, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectFollowCountProjectionPageSQL,
		cutoff, cursor.UpdatedAt, cursor.UpdatedAt, cursor.EntityID, cursor.UpdatedAt, cursor.EntityID, cursor.ShardID, limit)
	if err != nil {
		return nil, fmt.Errorf("query follow count projection page: %w", err)
	}
	defer rows.Close()
	result := make([]HGFollowCountProjection, 0, limit)
	for rows.Next() {
		var item HGFollowCountProjection
		if err := rows.Scan(&item.Cursor.UpdatedAt, &item.UserID, &item.Cursor.ShardID, &item.FollowerCount); err != nil {
			return nil, fmt.Errorf("scan follow count projection: %w", err)
		}
		item.Cursor.EntityID = item.UserID
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate follow count projection: %w", err)
	}
	return result, nil
}

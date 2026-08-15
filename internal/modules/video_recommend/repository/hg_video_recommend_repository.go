package VideoRecommendRepositoryPackage

import (
	VideoRecommendDtoPackage "MLC_GO/internal/modules/video_recommend/dto"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const hgMaxRecommendBatchSize = 256

// Repository 批量读取推荐候选的视频公开卡片，不执行全表扫描或深分页。
type Repository struct{ db *sql.DB }

// NewRepository 创建推荐视频只读仓储。
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// BatchGetPublicVideoCards 通过唯一业务 ID 有界批量补全卡片，结果 map 由 service 恢复召回顺序。
func (r *Repository) BatchGetPublicVideoCards(ctx context.Context, submissionIDs []string) (map[string]VideoRecommendDtoPackage.HGFeedItem, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("video recommend database cannot be nil")
	}
	if len(submissionIDs) == 0 {
		return map[string]VideoRecommendDtoPackage.HGFeedItem{}, nil
	}
	if len(submissionIDs) > hgMaxRecommendBatchSize {
		// IN 列表设置硬上限，避免异常调用放大 SQL 解析、执行计划和连接占用。
		return nil, fmt.Errorf("video recommend batch exceeds %d", hgMaxRecommendBatchSize)
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(submissionIDs)), ",")
	query := SQLQueriesPackage.SelectVideoRecommendCardsPrefixSQL + placeholders + SQLQueriesPackage.SelectVideoRecommendCardsSuffixSQL
	args := make([]any, len(submissionIDs))
	for i := range submissionIDs {
		args[i] = submissionIDs[i]
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query video recommend cards: %w", err)
	}
	defer rows.Close()

	items := make(map[string]VideoRecommendDtoPackage.HGFeedItem, len(submissionIDs))
	for rows.Next() {
		var item VideoRecommendDtoPackage.HGFeedItem
		var publishTime time.Time
		if err := rows.Scan(&item.SubmissionID, &item.VideoID, &item.AuthorID, &item.Title, &item.CoverURL, &item.Category,
			&item.Description, &item.Duration, &item.FilePath, &publishTime); err != nil {
			return nil, fmt.Errorf("scan video recommend card: %w", err)
		}
		item.PublishTime = publishTime.UTC().Format(time.RFC3339Nano)
		items[item.SubmissionID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate video recommend cards: %w", err)
	}
	return items, nil
}

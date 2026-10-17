package repository

import (
	"MLC_GO/internal/events"
	CrawlerEventPackage "MLC_GO/internal/events/crawler"
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	"MLC_GO/internal/outbox"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// WithHGVideoPipeline 在装配阶段启用可靠详情发现；同一仓储发布后禁止并发修改开关。
func (r *Repository) WithHGVideoPipeline(enabled bool) *Repository {
	r.hgVideoPipeline = enabled
	return r
}

// hgDiscoverVideos 在已有限量推荐事务中写采集身份及 Outbox，不执行任何外部 I/O。
func (r *Repository) hgDiscoverVideos(ctx context.Context, tx *sql.Tx, taskID, runID uint64, items []CrawlerPlatformPackage.HGRecommendation) error {
	if !r.hgVideoPipeline {
		return nil
	}
	for _, item := range items {
		if item.Platform != "bilibili" {
			continue
		}
		if !CrawlerPlatformPackage.HGValidBVID(item.ContentID) {
			return errors.New("详情发现的BV号无效")
		}
		id := uuid.NewString()
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGInsertVideoCollectionSQL, id, item.ContentID, taskID, runID, time.Now().UTC()); err != nil {
			return err
		}
		event, err := CrawlerEventPackage.HGEnvelope(CrawlerEventPackage.HGDiscovered, CrawlerEventPackage.HGVideoEvent{BVID: item.ContentID, CollectionID: id})
		if err != nil {
			return err
		}
		if err := hgSavePipelineEvent(ctx, tx, event); err != nil {
			return err
		}
	}
	return nil
}

// HGBackfillVideoPage 为最多100条历史列表补发缺失详情的发现事件，返回续页主键及投递数。
// 调用方必须串行推进游标并限制总页数；事务内仅写采集身份和 Outbox。
func (r *Repository) HGBackfillVideoPage(ctx context.Context, hgAfter uint64) (uint64, int, error) {
	if !r.hgVideoPipeline {
		return hgAfter, 0, errors.New("详情流水线未启用")
	}
	hgRows, hgErr := r.db.QueryContext(ctx, SQLQueriesPackage.HGBackfillVideoPageSQL, hgAfter)
	if hgErr != nil {
		return hgAfter, 0, hgErr
	}
	hgNext := hgAfter
	hgItems := make([]CrawlerPlatformPackage.HGRecommendation, 0, 100)
	for hgRows.Next() {
		var hgID uint64
		var hgPlatform, hgBVID string
		var hgStored bool
		if hgErr = hgRows.Scan(&hgID, &hgPlatform, &hgBVID, &hgStored); hgErr != nil {
			hgRows.Close()
			return hgAfter, 0, hgErr
		}
		hgNext = hgID
		if hgPlatform == "bilibili" && !hgStored && CrawlerPlatformPackage.HGValidBVID(hgBVID) {
			hgItems = append(hgItems, CrawlerPlatformPackage.HGRecommendation{Platform: hgPlatform, ContentID: hgBVID})
		}
	}
	hgErr = hgRows.Err()
	hgRows.Close()
	if hgErr != nil {
		return hgAfter, 0, hgErr
	}
	if len(hgItems) == 0 {
		return hgNext, 0, nil
	}
	hgTx, hgErr := r.db.BeginTx(ctx, nil)
	if hgErr != nil {
		return hgAfter, 0, hgErr
	}
	defer hgTx.Rollback()
	if hgErr = r.hgDiscoverVideos(ctx, hgTx, 0, 0, hgItems); hgErr != nil {
		return hgAfter, 0, hgErr
	}
	if hgErr = hgTx.Commit(); hgErr != nil {
		return hgAfter, 0, hgErr
	}
	return hgNext, len(hgItems), nil
}

// HGCollectionStatus 按采集主键查询进度；缺失身份属于协议错误，不能自动新建。
func (r *Repository) HGCollectionStatus(ctx context.Context, id, bvid string) (string, error) {
	var storedBVID, status string
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.HGGetVideoCollectionSQL, id).Scan(&storedBVID, &status)
	if err != nil {
		return "", err
	}
	if storedBVID != bvid {
		return "", errors.New("采集身份与视频不一致")
	}
	return status, nil
}

// HGAdvanceVideo 在一个短事务内推进阶段并写下游 Outbox；重复阶段不会重复发布。
// 抓取和清洗均在调用前完成。入库阶段基础详情与统计一同提交；旧快照仅保留历史事件。
func (r *Repository) HGAdvanceVideo(ctx context.Context, item CrawlerEventPackage.HGVideoEvent, expected, next, reason string, event *events.EventEnvelope, detail *CrawlerModelPackage.HGVideoDetail) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var bvid, status string
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGLockVideoCollectionSQL, item.CollectionID).Scan(&bvid, &status); err != nil {
		return err
	}
	if bvid != item.BVID {
		return errors.New("采集身份与视频不一致")
	}
	if status != expected {
		return nil
	}
	if detail != nil {
		if detail.BVID != item.BVID || !detail.CollectedAt.Equal(item.CollectedAt) {
			return errors.New("入库快照与采集身份不一致")
		}
		if err = CrawlerPlatformPackage.HGValidateVideoDetail(*detail); err != nil {
			return err
		}
		base := *detail
		base.Stats = CrawlerModelPackage.HGVideoStats{}
		body, marshalErr := json.Marshal(base)
		if marshalErr != nil {
			return marshalErr
		}
		stats, marshalErr := json.Marshal(detail.Stats)
		if marshalErr != nil {
			return marshalErr
		}
		if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGUpsertVideoDetailSQL, bvid, body, detail.CollectedAt, item.CollectionID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGUpsertVideoStatsSQL, bvid, stats, detail.CollectedAt, item.CollectionID); err != nil {
			return err
		}
	}
	if event != nil {
		if err = hgSavePipelineEvent(ctx, tx, *event); err != nil {
			return err
		}
	}
	hgResult, err := tx.ExecContext(ctx, SQLQueriesPackage.HGAdvanceVideoCollectionSQL, next, reason, item.CollectionID, expected)
	if err != nil {
		return err
	}
	if hgAffected, hgErr := hgResult.RowsAffected(); hgErr != nil || hgAffected != 1 {
		return errors.New("采集阶段更新未命中唯一记录")
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交采集阶段: %w", err)
	}
	return nil
}

// hgSavePipelineEvent 复用统一 Outbox，阶段名唯一映射到专用 topic。
func hgSavePipelineEvent(ctx context.Context, tx *sql.Tx, event events.EventEnvelope) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return outbox.NewRepository(nil, CrawlerEventPackage.HGTopicPrefix+event.EventName).SaveEnvelopeExec(ctx, tx, event, body)
}

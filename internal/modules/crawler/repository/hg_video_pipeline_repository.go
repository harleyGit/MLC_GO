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

// hgDiscoverVideos 为发现的 B 站视频创建采集记录和事件
/*执行流程：
推荐列表采集
      |
      v
hgDiscoverVideos()
      |
      +-- 插入视频采集记录
      |
      +-- 保存 discovered 事件到 Outbox
      |
      v
事务提交
      |
      v
Outbox Publisher
      |
      v
Kafka
      |
      v
HGStageConsumer.Handle()
      |
      v
HGAdvanceVideo()
*/
func (r *Repository) hgDiscoverVideos(ctx context.Context, tx *sql.Tx, taskID, runID uint64, items []CrawlerPlatformPackage.HGRecommendation) error {
	if !r.hgVideoPipeline { //如果视频详情采集流水线未开启，直接返回，不进行处理
		return nil
	}
	for _, item := range items { // 遍历推荐视频
		if item.Platform != "bilibili" { // 非 B 站视频，跳过
			continue
		}
		if !CrawlerPlatformPackage.HGValidBVID(item.ContentID) {
			return errors.New("详情发现的BV号无效")
		}
		// 为每次视频采集创建一个 UUID
		id := uuid.NewString()
		// 使用当前数据库事务 tx 插入一条采集记录。
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGInsertVideoCollectionSQL, id, item.ContentID, taskID, runID, time.Now().UTC()); err != nil {
			return err
		}

		// 创建一条表示“发现新视频”的事件。
		// 注意：CollectionID 表示一次采集身份，BVID 表示视频身份。
		// 同一个视频可以在不同时间被多次采集，因此可能对应多个 CollectionID。
		event, err := CrawlerEventPackage.HGEnvelope(CrawlerEventPackage.HGDiscovered, CrawlerEventPackage.HGVideoEvent{BVID: item.ContentID, CollectionID: id})
		if err != nil {
			return err
		}
		// 将事件保存到 Outbox 表，等待后续发送到 Kafka。
		if err := hgSavePipelineEvent(ctx, tx, event); err != nil {
			return err
		}
	}
	return nil
}

// HGBackfillVideoPage 分页补录历史视频，创建遗漏的采集任务。这个方法用于处理以前发现过，但还没有完成视频详情存储的视频。
// 为最多100条历史列表补发缺失详情的发现事件，返回续页主键及投递数。
//
//	@param ctx
//	@param hgAfter 是游标，通常表示上一页最后一条记录的自增 ID
//	@return uint64
//	@return int
//	@return error
func (r *Repository) HGBackfillVideoPage(ctx context.Context, hgAfter uint64) (uint64, int, error) {
	if !r.hgVideoPipeline {
		return hgAfter, 0, errors.New("详情流水线未启用")
	}

	// 查询一页历史数据
	hgRows, hgErr := r.db.QueryContext(ctx, SQLQueriesPackage.HGBackfillVideoPageSQL, hgAfter)
	if hgErr != nil {
		return hgAfter, 0, hgErr
	}
	hgNext := hgAfter
	hgItems := make([]CrawlerPlatformPackage.HGRecommendation, 0, 100)
	// 逐条读取数据库结果
	for hgRows.Next() {
		var hgID uint64               //记录 ID
		var hgPlatform, hgBVID string // hgBVID：视频 BVID。hgStored：是否已经存储详情。
		var hgStored bool
		if hgErr = hgRows.Scan(&hgID, &hgPlatform, &hgBVID, &hgStored); hgErr != nil {
			hgRows.Close()
			return hgAfter, 0, hgErr
		}
		// 注意，这里不论该记录是否需要补录，都会推进游标，避免下一页重复扫描。
		hgNext = hgID
		// 补录满足以下条件的视频：平台是 B 站、视频详情尚未存储、BVID 合法
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
	// 开始事务并创建采集任务
	hgTx, hgErr := r.db.BeginTx(ctx, nil)
	if hgErr != nil {
		return hgAfter, 0, hgErr
	}
	// 如果没有成功提交，则回滚事务；成功提交后，再调用 Rollback() 不会撤销已提交的数据
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

// HGAdvanceVideo 通过数据库事务安全推进采集阶段，并保存视频数据。这个方法是整套视频采集流水线的核心。
// 它负责保证：视频详情写入、Outbox 事件保存、采集状态更新在同一个数据库事务中完成。
// 在一个短事务内推进阶段并写下游 Outbox；重复阶段不会重复发布。
// 抓取和清洗均在调用前完成。入库阶段基础详情与统计一同提交；旧快照仅保留历史事件。
/*执行流程：
HGAdvanceVideo()
      |
      +-- 锁定采集记录
      +-- 验证当前状态
      +-- 保存视频详情及统计
      +-- 保存下一阶段事件
      +-- 更新采集状态
      |
      v
事务提交
*/
func (r *Repository) HGAdvanceVideo(ctx context.Context, item CrawlerEventPackage.HGVideoEvent, expected, next, reason string, event *events.EventEnvelope, detail *CrawlerModelPackage.HGVideoDetail) error {

	// sql.LevelReadCommitted 表示使用 读已提交隔离级别。简单理解：普通读取不会看到其他事务尚未提交的数据
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
	// 然后检查数据库当前状态是否符合期望。比如：当前消息可能已经被处理过，或者任务已经进入其他阶段，因此跳过
	if status != expected {
		return nil
	}
	if detail != nil { //只有传入了视频详情，才执行保存逻辑
		if detail.BVID != item.BVID || !detail.CollectedAt.Equal(item.CollectedAt) {
			return errors.New("入库快照与采集身份不一致")
		}
		if err = CrawlerPlatformPackage.HGValidateVideoDetail(*detail); err != nil {
			return err
		}
		/* base := *detail 创建结构体的值拷贝，随后将副本的 Stats 清空。
		目的：将视频基础信息和视频统计信息分开保存。
		HGVideoDetail
			|
			+-- 视频基础信息
			|     标题、作者、封面、时长
			|
			+-- 视频统计信息
				播放、点赞、收藏、评论
		*/
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
		// 写入数据库，保存视频基础详情，保存视频统计快照。Upsert通常表示存在则更新，不存在则插入
		if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGUpsertVideoDetailSQL, bvid, body, detail.CollectedAt, item.CollectionID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGUpsertVideoStatsSQL, bvid, stats, detail.CollectedAt, item.CollectionID); err != nil {
			return err
		}
	}
	if event != nil {
		// 保存下一阶段事件，如果存在下一阶段事件，将其写入 Outbox 表。
		if err = hgSavePipelineEvent(ctx, tx, *event); err != nil {
			return err
		}
	}
	// 更新采集状态，只有状态符合预期才允许更新
	hgResult, err := tx.ExecContext(ctx, SQLQueriesPackage.HGAdvanceVideoCollectionSQL, next, reason, item.CollectionID, expected)
	if err != nil {
		return err
	}
	// 检查实际影响数据库行数
	if hgAffected, hgErr := hgResult.RowsAffected(); hgErr != nil || hgAffected != 1 {
		return errors.New("采集阶段更新未命中唯一记录")
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交采集阶段: %w", err)
	}
	return nil
}

// hgSavePipelineEvent 将 Kafka 待发送事件保存到 Outbox 表
func hgSavePipelineEvent(ctx context.Context, tx *sql.Tx, event events.EventEnvelope) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	// 保存事件到 Outbox 表
	return outbox.NewRepository(nil, CrawlerEventPackage.HGTopicPrefix+event.EventName).SaveEnvelopeExec(ctx, tx, event, body)
}

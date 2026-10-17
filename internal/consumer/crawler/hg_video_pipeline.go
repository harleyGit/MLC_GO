// Package crawler 实现外部视频发现、抓取、清洗、入库与历史投影。
package crawler

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	CrawlerEventPackage "MLC_GO/internal/events/crawler"
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	CrawlerServicePackage "MLC_GO/internal/modules/crawler/service"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// HGStore 负责采集阶段、幂等及 Outbox 原子提交，禁止事务内调用 HTTP。
type HGStore interface {
	HGCollectionStatus(context.Context, string, string) (string, error)
	HGAdvanceVideo(context.Context, CrawlerEventPackage.HGVideoEvent, string, string, string, *events.EventEnvelope, *CrawlerModelPackage.HGVideoDetail) error
}

// HGHistoryStore 批量写历史快照；返回成功表示已收到 ClickHouse 持久化确认。
type HGHistoryStore interface {
	HGStoreCrawlerHistory(context.Context, []HGHistoryRow) error
}

// HGHistoryRow 是 ClickHouse 摄入协议，按采集身份去重而非累计求和。
type HGHistoryRow struct {
	CollectionID string `json:"collection_id"`  // 稳定采集UUID。
	BVID         string `json:"bvid"`           // 原站视频标识。
	CollectedAt  int64  `json:"collected_at"`   // UTC Unix毫秒，作为同次采集的稳定分区时间。
	View         int64  `json:"view_count"`     // 原站累计播放次数。
	Like         int64  `json:"like_count"`     // 原站累计点赞次数。
	Coin         int64  `json:"coin_count"`     // 原站累计投币枚数。
	Favorite     int64  `json:"favorite_count"` // 原站累计收藏次数。
	Share        int64  `json:"share_count"`    // 原站累计分享次数。
	Reply        int64  `json:"reply_count"`    // 原站累计评论条数。
	Danmaku      int64  `json:"danmaku_count"`  // 原站累计弹幕条数。
}

// HGStageConsumer 每个阶段一个独立消费组；抓取限制单进程1请求/秒且无并行外部请求。
type HGStageConsumer struct {
	stage   string                                   // 必须匹配专用 topic 的事件名。
	store   HGStore                                  // MySQL权威写入及阶段状态。
	http    CrawlerServicePackage.HGTaskHTTPExecutor // 复用已有SSRF防护客户端。
	history HGHistoryStore                           // 仅历史阶段使用。
	limiter *rate.Limiter                            // 抓取阶段的进程级限速器。
}

// NewHGStageConsumer 只构造处理器，生命周期由现有 Kafka runtime 管理。
func NewHGStageConsumer(stage string, store HGStore, httpClient CrawlerServicePackage.HGTaskHTTPExecutor, history HGHistoryStore) *HGStageConsumer {
	return &HGStageConsumer{stage: stage, store: store, http: httpClient, history: history, limiter: rate.NewLimiter(rate.Limit(1), 1)}
}

// hgDecode 校验专用消息身份和载荷上限，错误进入可靠DLQ后才允许推进offset。
func (c *HGStageConsumer) hgDecode(e events.EventEnvelope) (CrawlerEventPackage.HGVideoEvent, error) {
	var item CrawlerEventPackage.HGVideoEvent
	if e.Version != 1 || e.EventName != c.stage || len(e.Payload) > 2<<20 {
		return item, errors.New("采集事件协议或大小无效")
	}
	if err := json.Unmarshal(e.Payload, &item); err != nil {
		return item, errors.New("采集事件JSON无效")
	}
	if _, err := uuid.Parse(item.CollectionID); err != nil {
		return item, errors.New("采集身份无效")
	}
	if !CrawlerPlatformPackage.HGValidBVID(item.BVID) || e.EventKey != "bilibili:"+item.BVID || e.EventID != CrawlerEventPackage.HGEventID(item.CollectionID, c.stage) {
		return item, errors.New("采集事件身份不匹配")
	}
	return item, nil
}

// Handle 完成当前阶段持久化后返回成功，基础设施负责手动offset提交及重放。
func (c *HGStageConsumer) Handle(ctx context.Context, e events.EventEnvelope) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	item, err := c.hgDecode(e)
	if err != nil {
		return HGKafkaPackage.HGNewTerminalError(err)
	}
	if c.stage == CrawlerEventPackage.HGStored {
		return c.hgHistory(ctx, []consumer.DeliveredEnvelope{{Envelope: e}})
	}
	expected := map[string]string{CrawlerEventPackage.HGDiscovered: "pending", CrawlerEventPackage.HGRaw: "raw", CrawlerEventPackage.HGCleaned: "cleaned"}[c.stage]
	status, err := c.store.HGCollectionStatus(ctx, item.CollectionID, item.BVID)
	if err != nil {
		return err
	}
	if status == "failed" {
		return HGKafkaPackage.HGNewTerminalError(errors.New("采集已失败，保留消息用于人工恢复"))
	}
	if status != expected {
		return nil
	}
	var detail *CrawlerModelPackage.HGVideoDetail
	var next, stage string
	switch c.stage {
	case CrawlerEventPackage.HGDiscovered:
		if err = c.limiter.Wait(ctx); err != nil {
			return err
		}
		response, fetchErr := c.http.Execute(ctx, CrawlerDtoPackage.HGDebugRequest{Method: "GET", URL: "https://api.bilibili.com/x/web-interface/view?bvid=" + item.BVID, TimeoutMS: 10000})
		if fetchErr != nil {
			return fetchErr
		}
		if response.StatusCode == 403 || response.StatusCode == 429 {
			// 限制响应停止该次采集，避免Kafka重试持续冲击上游；人工重跑创建新的采集身份。
			c.limiter.SetLimit(rate.Every(30 * time.Second))
			return c.hgFail(ctx, item, expected, "上游限制访问，请降低采集频率后重跑")
		}
		if response.StatusCode >= 500 {
			return fmt.Errorf("详情上游HTTP %d", response.StatusCode)
		}
		if response.StatusCode != 200 {
			return c.hgFail(ctx, item, expected, "详情上游拒绝请求")
		}
		// 为信封和JSON转义预留空间，使消息及DLQ包装均低于4MiB topic上限。
		if len(response.Body) > 1<<20 {
			return c.hgFail(ctx, item, expected, "详情原始响应超过1MiB消息预算")
		}
		var code struct {
			Code int `json:"code"`
		} // 上游业务状态；完整字段清洗由下阶段负责。
		if err = json.Unmarshal(response.Body, &code); err != nil {
			return c.hgFail(ctx, item, expected, "详情响应不是JSON")
		}
		if code.Code != 0 {
			return c.hgFail(ctx, item, expected, fmt.Sprintf("详情上游业务码%d", code.Code))
		}
		item.Data = response.Body
		item.CollectedAt = time.Now().UTC().Truncate(time.Microsecond)
		next = "raw"
		stage = CrawlerEventPackage.HGRaw
	case CrawlerEventPackage.HGRaw:
		cleaned, cleanErr := CrawlerPlatformPackage.HGCleanBilibiliDetail(item.Data, item.BVID, item.CollectedAt)
		if cleanErr != nil {
			return c.hgFail(ctx, item, expected, "详情清洗校验失败")
		}
		item.Data, err = json.Marshal(cleaned)
		if err != nil {
			return err
		}
		next = "cleaned"
		stage = CrawlerEventPackage.HGCleaned
	case CrawlerEventPackage.HGCleaned:
		var cleaned CrawlerModelPackage.HGVideoDetail
		if err = json.Unmarshal(item.Data, &cleaned); err != nil {
			return c.hgFail(ctx, item, expected, "清洗快照JSON无效")
		}
		if err = hgValidateSnapshot(item, cleaned); err != nil {
			return c.hgFail(ctx, item, expected, "清洗快照身份或字段无效")
		}
		detail = &cleaned
		next = "stored"
		stage = CrawlerEventPackage.HGStored
	default:
		return HGKafkaPackage.HGNewTerminalError(errors.New("未知采集阶段"))
	}
	event, err := CrawlerEventPackage.HGEnvelope(stage, item)
	if err != nil {
		return err
	}
	return c.store.HGAdvanceVideo(ctx, item, expected, next, "", &event, detail)
}

// hgFail 先可靠记录失败，再返回终止错误；DLQ未确认时源offset不会提交。
func (c *HGStageConsumer) hgFail(ctx context.Context, item CrawlerEventPackage.HGVideoEvent, expected, reason string) error {
	if err := c.store.HGAdvanceVideo(ctx, item, expected, "failed", reason, nil, nil); err != nil {
		return err
	}
	return HGKafkaPackage.HGNewTerminalError(errors.New(reason))
}

// HandleBatch 历史阶段批量摄入，其余阶段逐条执行；失败携带准确索引，不能越过失败位点。
func (c *HGStageConsumer) HandleBatch(ctx context.Context, items []consumer.DeliveredEnvelope) error {
	// 每批处理预算20秒；到期返回准确失败索引，后续消息留待下一次poll，避免阻塞重平衡。
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if c.stage == CrawlerEventPackage.HGStored {
		return c.hgHistory(ctx, items)
	}
	for i, item := range items {
		if err := c.Handle(ctx, item.Envelope); err != nil {
			return HGKafkaPackage.HGNewBatchRecordError(i, err)
		}
	}
	return nil
}

// hgHistory 一个分区最多500条；ClickHouse失败不推进offset，重试使用同一collection_id。
func (c *HGStageConsumer) hgHistory(ctx context.Context, items []consumer.DeliveredEnvelope) error {
	if len(items) > 500 {
		return errors.New("历史批次超过500条")
	}
	rows := make([]HGHistoryRow, 0, len(items))
	for i, e := range items {
		item, err := c.hgDecode(e.Envelope)
		var d CrawlerModelPackage.HGVideoDetail
		if err == nil {
			err = json.Unmarshal(item.Data, &d)
		}
		if err == nil {
			err = hgValidateSnapshot(item, d)
		}
		if err != nil {
			if len(rows) > 0 {
				if writeErr := c.history.HGStoreCrawlerHistory(ctx, rows); writeErr != nil {
					return writeErr
				}
			}
			return HGKafkaPackage.HGNewBatchRecordError(i, HGKafkaPackage.HGNewTerminalError(errors.New("历史快照校验失败")))
		}
		s := d.Stats
		rows = append(rows, HGHistoryRow{CollectionID: item.CollectionID, BVID: item.BVID, CollectedAt: item.CollectedAt.UnixMilli(), View: s.View, Like: s.Like, Coin: s.Coin, Favorite: s.Favorite, Share: s.Share, Reply: s.Reply, Danmaku: s.Danmaku})
	}
	if len(rows) == 0 {
		return nil
	}
	return c.history.HGStoreCrawlerHistory(ctx, rows)
}

// hgValidateSnapshot 防止载荷中的身份或时间与信封不一致，确保重放落入同一历史分区。
func hgValidateSnapshot(item CrawlerEventPackage.HGVideoEvent, d CrawlerModelPackage.HGVideoDetail) error {
	if d.BVID != item.BVID || !d.CollectedAt.Equal(item.CollectedAt) {
		return errors.New("快照身份或时间不匹配")
	}
	return CrawlerPlatformPackage.HGValidateVideoDetail(d)
}

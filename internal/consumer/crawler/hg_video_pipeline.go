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

// HGStageConsumer 当前 Kafka 消费者对象（每个阶段一个独立消费组）；抓取限制单进程1请求/秒且无并行外部请求。
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
//
//	@param e 接收到的事件消息
//	@return CrawlerEventPackage.HGVideoEvent 解析后的视频采集事
//	@return error 解析或验证失败时返回错误
func (c *HGStageConsumer) hgDecode(e events.EventEnvelope) (CrawlerEventPackage.HGVideoEvent, error) {
	var item CrawlerEventPackage.HGVideoEvent
	// 协议版本必须为 1；事件名称必须匹配当前消费阶段；JSON 数据不能超过 2 MiB
	if e.Version != 1 || e.EventName != c.stage || len(e.Payload) > 2<<20 {
		return item, errors.New("采集事件协议或大小无效")
	}
	if err := json.Unmarshal(e.Payload, &item); err != nil {
		return item, errors.New("采集事件JSON无效")
	}
	if _, err := uuid.Parse(item.CollectionID); err != nil {
		return item, errors.New("采集身份无效")
	}
	// 作用： 防止事件外层标识和 JSON 内部数据不一致，避免不同视频、不同采集阶段的数据混淆。
	// 注意：这里是业务一致性验证，并不是密码学签名验证，不能单独防止恶意伪造。
	if !CrawlerPlatformPackage.HGValidBVID(item.BVID) || e.EventKey != "bilibili:"+item.BVID || e.EventID != CrawlerEventPackage.HGEventID(item.CollectionID, c.stage) {
		return item, errors.New("采集事件身份不匹配")
	}
	return item, nil
}

// Handle 该方法是 Kafka 视频爬虫消费者的核心业务处理方法。
// 主要作用：根据当前采集阶段，执行视频详情抓取、数据清洗、数据存储，并将任务推进到下一个阶段。
/* 执行流程：
Kafka 接收到视频采集事件
             |
             v
      Handle(ctx, e)
             |
             v
    设置15秒处理超时
             |
             v
    hgDecode() 解析事件
             |
             v
    查询数据库当前状态
             |
             v
    判断当前采集阶段
             |
             +-- discovered（发现）
             |      |
             |      v
             |   HTTP请求B站视频详情
             |      |
             |      v
             |     raw
             |
             +-- raw（原始数据）
             |      |
             |      v
             |   清洗、校验视频数据
             |      |
             |      v
             |    cleaned
             |
             +-- cleaned（已清洗）
             |      |
             |      v
             |   验证数据并准备入库
             |      |
             |      v
             |    stored
             |
             +-- stored（已存储）
                    |
                    v
              hgHistory()
                    |
                    v
              历史处理流程
*/
func (c *HGStageConsumer) Handle(ctx context.Context, e events.EventEnvelope) error {

	// 1. 设置处理超时
	/* 给本次 Kafka 消费处理设置 15 秒超时。
	- 超过 15 秒，ctx 会被取消。
	- defer cancel() 在方法退出时释放相关资源。
	- 如果上层 ctx 提前取消，也会立即传递下来。
	注意：超时不会强制中断所有代码，具体操作需要支持并正确响应 ctx。
	*/
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// 2. 解析 Kafka 消息
	/* 调用hgDecode()，验证事件格式和身份。
	HGNewTerminalError(err) 表示将错误标记为终止性错误。
	通常用于告诉上层消费者：这类错误不适合继续自动重试。
	实际是否进入 DLQ（死信队列），由外层 Kafka 消费框架决定。
	*/
	item, err := c.hgDecode(e)
	if err != nil {
		return HGKafkaPackage.HGNewTerminalError(err)
	}
	// 3.特殊处理 stored 阶段； 如果当前是 stored 阶段，直接交给 hgHistory() 处理。
	if c.stage == CrawlerEventPackage.HGStored {
		return c.hgHistory(ctx, []consumer.DeliveredEnvelope{{Envelope: e}})
	}
	// 4. 查询当前数据库状态
	expected := map[string]string{CrawlerEventPackage.HGDiscovered: "pending", CrawlerEventPackage.HGRaw: "raw", CrawlerEventPackage.HGCleaned: "cleaned"}[c.stage]
	// 根据采集任务 ID 和视频 BVID 查询数据库当前状态。
	status, err := c.store.HGCollectionStatus(ctx, item.CollectionID, item.BVID)
	if err != nil {
		return err
	}
	// failed：任务已经失败，不再自动执行
	/* 为什么需要下面2个判断？
	Kafka 可能重复投递消息。例如数据库已经是 cleaned，却再次收到 discovered 消息。
	此时跳过，避免重复抓取。
	这属于幂等处理的一部分，但完整的幂等保障还依赖数据库状态更新的实现。
	*/
	if status == "failed" {
		return HGKafkaPackage.HGNewTerminalError(errors.New("采集已失败，保留消息用于人工恢复"))
	}
	// status != expected：数据库状态与当前消息不匹配，直接跳过。
	if status != expected {
		return nil
	}
	var detail *CrawlerModelPackage.HGVideoDetail
	var next, stage string
	switch c.stage {
	case CrawlerEventPackage.HGDiscovered: //抓取视频详情
		if err = c.limiter.Wait(ctx); err != nil { // 第一步：限流；等待限流器允许发送请求，避免短时间请求过多。
			return err
		}
		// 第二步：请求 B 站视频详情接口； 发送 GET 请求，获取视频详情 JSON。单次 HTTP 请求配置了 10 秒超时。
		response, fetchErr := c.http.Execute(ctx, CrawlerDtoPackage.HGDebugRequest{Method: "GET", URL: "https://api.bilibili.com/x/web-interface/view?bvid=" + item.BVID, TimeoutMS: 10000})
		if fetchErr != nil {
			return fetchErr
		}
		/* 第三步：处理 HTTP 状态码
		200 ： 继续处理
		403 / 429：降低请求频率，标记采集失败
		500 及以上：返回错误，交给上层处理
		其他非 200：标记采集失败
		*/
		if response.StatusCode == 403 || response.StatusCode == 429 {
			// 限制响应停止该次采集，避免Kafka重试持续冲击上游；人工重跑创建新的采集身份。
			// 将限流速率调整为平均每 30 秒允许一个请求
			c.limiter.SetLimit(rate.Every(30 * time.Second))
			return c.hgFail(ctx, item, expected, "上游限制访问，请降低采集频率后重跑")
		}
		if response.StatusCode >= 500 {
			return fmt.Errorf("详情上游HTTP %d", response.StatusCode)
		}
		if response.StatusCode != 200 {
			return c.hgFail(ctx, item, expected, "详情上游拒绝请求")
		}
		// 第四步：检查响应数据。B 站响应可能携带体积很大的合集等无关字段，
		// 先保留清洗阶段所需的协议字段，再按 Kafka 1 MiB 单消息预算校验。
		var code struct {
			Code int `json:"code"`
		} // 上游业务状态；完整字段清洗由下阶段负责。
		if err = json.Unmarshal(response.Body, &code); err != nil {
			return c.hgFail(ctx, item, expected, "详情响应不是JSON")
		}
		// 解析B站业务返回码，要求code.Code == 0
		if code.Code != 0 {
			return c.hgFail(ctx, item, expected, fmt.Sprintf("详情上游业务码%d", code.Code))
		}
		hgCompacted, hgCompactErr := CrawlerPlatformPackage.HGCompactBilibiliDetail(response.Body)
		if hgCompactErr != nil {
			return c.hgFail(ctx, item, expected, "详情响应裁剪失败")
		}
		// 第五步：保存原始数据并设置下一阶段; 表示抓取完成，准备进入 raw 阶段
		item.Data = hgCompacted
		item.CollectedAt = time.Now().UTC().Truncate(time.Microsecond)
		next = "raw"
		stage = CrawlerEventPackage.HGRaw
	case CrawlerEventPackage.HGRaw: // 阶段二： HGRaw（清洗原始数据）
		// 调用清洗函数，将 B 站原始 JSON 转换为系统自己的视频数据模型。
		cleaned, cleanErr := CrawlerPlatformPackage.HGCleanBilibiliDetail(item.Data, item.BVID, item.CollectedAt)
		if cleanErr != nil {
			return c.hgFail(ctx, item, expected, "详情清洗校验失败")
		}
		// 将清洗后的Go结构体重新转换成JSON
		item.Data, err = json.Marshal(cleaned)
		if err != nil {
			return err
		}
		// 准备进入下一个阶段
		next = "cleaned"
		stage = CrawlerEventPackage.HGCleaned
	case CrawlerEventPackage.HGCleaned: //  阶段三：HGCleaned（验证并准备存储）

		// 先将清洗后的 JSON 解析为业务模型：
		var cleaned CrawlerModelPackage.HGVideoDetail
		if err = json.Unmarshal(item.Data, &cleaned); err != nil {
			return c.hgFail(ctx, item, expected, "清洗快照JSON无效")
		}
		// 检查视频身份和关键业务字段是否符合要求。
		if err = hgValidateSnapshot(item, cleaned); err != nil {
			return c.hgFail(ctx, item, expected, "清洗快照身份或字段无效")
		}
		//表示视频详情数据已经准备好写入数据库。
		detail = &cleaned
		next = "stored"
		stage = CrawlerEventPackage.HGStored
	default:
		return HGKafkaPackage.HGNewTerminalError(errors.New("未知采集阶段"))
	}
	// 根据下一阶段的名称和数据，构建新的 Kafka 事件信封。
	event, err := CrawlerEventPackage.HGEnvelope(stage, item)
	if err != nil {
		return err
	}
	// 负责推进数据库中的采集状态，并处理下一阶段事件。
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

// hgHistory 批量提取视频统计数据并保存历史记录【视频统计数据的批量历史入库】
// 一个分区最多500条；ClickHouse失败不推进offset，重试使用同一collection_id。
func (c *HGStageConsumer) hgHistory(ctx context.Context, items []consumer.DeliveredEnvelope) error {

	// 每批最多处理 500 条 Kafka 事件，防止单批数据过大。
	if len(items) > 500 {
		return errors.New("历史批次超过500条")
	}
	rows := make([]HGHistoryRow, 0, len(items))
	// 遍历 Kafka 消息
	// i：当前消息在批次中的索引，从 0 开始；e：当前 Kafka 消息。
	for i, e := range items {
		// item：解析出来的采集事件
		item, err := c.hgDecode(e.Envelope)
		// d：存放视频详情数据
		var d CrawlerModelPackage.HGVideoDetail
		if err == nil {
			err = json.Unmarshal(item.Data, &d)
		}
		if err == nil {
			// 验证视频快照合法性
			err = hgValidateSnapshot(item, d)
		}
		// 遇到错误时的处理
		if err != nil {
			if len(rows) > 0 {
				if writeErr := c.history.HGStoreCrawlerHistory(ctx, rows); writeErr != nil {
					return writeErr
				}
			}
			// 比如有5条数据，处理前3条成功，处理到第4条时失败，则返回第4条的索引（也就是批次中的位置，方便上层kafka批处理逻辑定位失败记录）和错误。第五条不会再有本次循环中进行处理
			return HGKafkaPackage.HGNewBatchRecordError(i, HGKafkaPackage.HGNewTerminalError(errors.New("历史快照校验失败")))
		}
		s := d.Stats
		// 将视频详情中的统计数据转换为历史记录。
		rows = append(rows, HGHistoryRow{CollectionID: item.CollectionID, BVID: item.BVID, CollectedAt: item.CollectedAt.UnixMilli(), View: s.View, Like: s.Like, Coin: s.Coin, Favorite: s.Favorite, Share: s.Share, Reply: s.Reply, Danmaku: s.Danmaku})
	}
	if len(rows) == 0 {
		return nil
	}

	// 批量写入数据库并返回，否则一次性批量写入数据库，避免每条消息都单独执行一次数据库写入。
	return c.history.HGStoreCrawlerHistory(ctx, rows)
}

// hgValidateSnapshot 防止载荷中的身份或时间与信封不一致，确保重放落入同一历史分区。
func hgValidateSnapshot(item CrawlerEventPackage.HGVideoEvent, d CrawlerModelPackage.HGVideoDetail) error {
	if d.BVID != item.BVID || !d.CollectedAt.Equal(item.CollectedAt) {
		return errors.New("快照身份或时间不匹配")
	}
	return CrawlerPlatformPackage.HGValidateVideoDetail(d)
}

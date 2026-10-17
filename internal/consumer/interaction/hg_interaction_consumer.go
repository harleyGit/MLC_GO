package interaction

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	InteractionEventsPackage "MLC_GO/internal/events/interaction"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"encoding/json"
	"fmt"
)

// PersistedEvent 表示经过解析后、准备持久化到数据库的事件数据。
// 包含事件基本信息、业务数据、Kafka 消息元数据和原始消息内容。
type PersistedEvent struct {
	// 事件基本信息
	EventID   string // 事件唯一 ID，用于事件追踪和幂等去重
	EventName string // 事件名称，用于区分不同的事件类型，比如： 视频点赞、用户关注关系变化
	EventKey  string // 事件业务标识，用于关联或定位业务对象

	// 视频互动业务字段
	ActorUserID  string `json:"actorUserId"`  // 操作用户 ID，例如点赞视频的用户
	SubmissionID string `json:"submissionId"` // 投稿或视频 ID，表示被操作的内容
	Action       string `json:"action"`       // 操作类型，例如点赞、收藏、关注等
	Active       bool   `json:"active"`       // 操作是否处于生效状态，例如 true 表示已点赞，false 表示取消点赞
	Quantity     int    `json:"quantity"`     // 操作数量或变化数量，具体含义由业务协议决定

	// 用户关注业务字段
	FollowID   string `json:"followId"`   // 关注关系的唯一 ID
	FollowerID string `json:"followerId"` // 关注者用户 ID，即主动关注别人的用户
	FolloweeID string `json:"followeeId"` // 被关注者用户 ID，即被别人关注的用户

	// Kafka 消息元数据，比如：Go 服务使用 Kafka 接收用户点赞事件。
	/*
			Kafka 集群
		    |
		    +-- Topic: video-interaction
		          |
		          +-- Partition 0
		          |     +-- Offset 0 → 用户A点赞
		          |     +-- Offset 1 → 用户B点赞
		          |     +-- Offset 2 → 用户C点赞
		          |
		          +-- Partition 1
		          |     +-- Offset 0 → 用户D点赞
		          |     +-- Offset 1 → 用户E点赞
		          |
		          +-- Partition 2
		                +-- Offset 0 → 用户F点赞
		                +-- Offset 1 → 用户G点赞
	*/
	KafkaTopic     string // Kafka Topic 名称，表示消息来自哪个主题
	KafkaPartition int32  // Kafka 分区编号，表示消息位于哪个分区
	KafkaOffset    int64  // Kafka 消息偏移量，表示消息在所属分区中的位置

	// 原始消息
	Payload string // Kafka 消息的原始 JSON 内容，用于审计、排查和事件重放
}

type EventStore interface {

	// ApplyEvent  是数据存储执行者。
	//	@param context.Context
	//	@param PersistedEvent
	//	@return error
	ApplyEvent(context.Context, PersistedEvent) error

	// ApplyEvents 按固定上限拆分事务，减少逐条开事务开销且避免大事务长时间持锁。
	// 分批处理事件
	//	@param context.Context
	//	@param []PersistedEvent
	//	@return error
	ApplyEvents(context.Context, []PersistedEvent) error
}

type Consumer struct{ store EventStore }

func NewConsumer(store EventStore) *Consumer { return &Consumer{store: store} }

// Handle Kafka 消费者（Consumer）处理消息事件，并将事件保存到数据库
// 事件处理入口，决定是否处理消息，并调用数据库保存。
//	@param ctx
//	@param envelope
//	@return error
/* 执行流程图：
Kafka 收到一条消息
        |
        v
   Handle()
   消息处理入口
        |
        v
   hgMapEvent()
        |
        |-- 1. 判断是否为关注的事件
        |       |
        |       +-- 否 --> 忽略消息
        |
        |-- 2. 检查 store 是否存在
        |
        |-- 3. 获取 Kafka 元数据
        |       Topic / Partition / Offset
        |
        |-- 4. 解析 JSON 消息
        |
        |-- 5. 组装 PersistedEvent
        |
        v
  返回 event, true, nil
        |
        v
   store.ApplyEvent()
        |
        v
  保存或更新数据库
*/
func (c *Consumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {
	event, interested, err := c.hgMapEvent(ctx, envelope)
	if err != nil || !interested {
		// Handle() 返回错误，由上层消费者框架决定是否重试等操作
		return err
	}
	// 将解析好的事件交给存储层处理。
	return c.store.ApplyEvent(ctx, event)
}

// HandleBatch 将同一 Kafka 分区的有界消息批次一次交给 MySQL repository。
func (c *Consumer) HandleBatch(ctx context.Context, delivered []consumer.DeliveredEnvelope) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("interaction event store cannot be nil")
	}
	eventsToPersist := make([]PersistedEvent, 0, len(delivered))
	for index, item := range delivered {
		itemCtx := consumer.WithDelivery(ctx, item.Delivery)
		event, interested, err := c.hgMapEvent(itemCtx, item.Envelope)
		if err != nil {
			if len(eventsToPersist) > 0 {
				if applyErr := c.store.ApplyEvents(ctx, eventsToPersist); applyErr != nil {
					return applyErr
				}
			}
			return HGKafkaPackage.HGNewBatchRecordError(index, HGKafkaPackage.HGNewTerminalError(err))
		}
		if interested {
			eventsToPersist = append(eventsToPersist, event)
		}
	}
	if len(eventsToPersist) == 0 {
		return nil
	}
	return c.store.ApplyEvents(ctx, eventsToPersist)
}

// hgMapEvent() 是数据转换器
// hgMapEvent 事件解析与转换，检查消息是否合法，将 Kafka 消息转换成数据库需要的数据结构。
//
//	@param ctx
//	@param envelope Kafka 订阅的消息
//	@return PersistedEvent 解析完成的事件数据
//	@return bool 是否需要处理该事件
//	@return error 解析过程中是否发生错误
func (c *Consumer) hgMapEvent(ctx context.Context, envelope events.EventEnvelope) (PersistedEvent, bool, error) {

	// 若事件不是视频互动、用户关注，事件直接取消
	if envelope.EventName != InteractionEventsPackage.VideoInteractionChangedEventName && envelope.EventName != InteractionEventsPackage.UserFollowChangedEventName {
		// 消息不需要处理， 调用这个方法的Handle直接返回 nil，不保存数据库。
		return PersistedEvent{}, false, nil
	}
	// 检查数据库存储对象；防止 Consumer 或 store 为空，导致后续调用出错。
	if c == nil || c.store == nil {
		// 消息解释失败
		return PersistedEvent{}, false, fmt.Errorf("interaction event store cannot be nil")
	}
	// 获取 Kafka 消息元数据
	/* 从 ctx 中获取 Kafka 消息的投递信息。
	例如：
	Topic     = video_interaction
	Partition = 2
	Offset    = 105
	*/
	delivery, ok := consumer.DeliveryFromContext(ctx)
	if !ok || delivery.Topic == "" || delivery.Partition < 0 || delivery.Offset < 0 {
		return PersistedEvent{}, false, fmt.Errorf("interaction kafka delivery metadata is invalid")
	}
	event := PersistedEvent{EventID: envelope.EventID, EventName: envelope.EventName, EventKey: envelope.EventKey, KafkaTopic: delivery.Topic, KafkaPartition: delivery.Partition, KafkaOffset: delivery.Offset, Payload: string(envelope.Payload)}
	if envelope.EventName == InteractionEventsPackage.VideoInteractionChangedEventName {
		if err := json.Unmarshal(envelope.Payload, &event); err != nil {
			return PersistedEvent{}, false, fmt.Errorf("decode video interaction event: %w", err)
		}
	} else {
		if err := json.Unmarshal(envelope.Payload, &event); err != nil {
			return PersistedEvent{}, false, fmt.Errorf("decode follow event: %w", err)
		}
	}
	// json.Unmarshal 仅覆盖带 tag 的字段；恢复 Envelope 和 delivery 字段。
	// 	重新赋值是为了确保：
	// - 事件 ID、名称等来自 envelope。
	// - Kafka 元数据来自 delivery。
	// - 原始 JSON 内容来自 envelope.Payload。
	event.EventID, event.EventName, event.EventKey = envelope.EventID, envelope.EventName, envelope.EventKey

	event.KafkaTopic, event.KafkaPartition, event.KafkaOffset, event.Payload = delivery.Topic, delivery.Partition, delivery.Offset, string(envelope.Payload)
	// 消息解析成功
	return event, true, nil
}

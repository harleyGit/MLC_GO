package notification

import (
	"MLC_GO/internal/events"
	"context"
	"fmt"
	"math"
	"strconv"
)

// HGSendReadyEventName 是独立发送 topic 的事件名；旧 notification 消费组不可将它当作可提交的投递事件。
const HGSendReadyEventName = "notification.send.ready.v1"

// HGSendReadyEvent 只传递已持久化任务的游标与代次，不携带用户 ID、文案或变量。
// 消费方必须按任务主键、游标与代次在 MySQL 原子校验所有权；Kafka 事件不是授权或收件人快照。
type HGSendReadyEvent struct {
	events.EventMeta
	TaskID     string `json:"taskId"`     // 十进制任务 ID，必须可解析为正数 BIGINT。
	NextIndex  uint16 `json:"nextIndex"`  // 接收人数组位置，范围 0–99。
	Generation uint64 `json:"generation"` // MySQL 原子递增的分发代次；不能为 0。
}

// HGNewSendReadyEvent 构造单任务当前代次的最小事件；发送前仍须在同一事务写入 Outbox。
func HGNewSendReadyEvent(ctx context.Context, taskID uint64, index uint16, generation uint64) (HGSendReadyEvent, error) {
	event := HGSendReadyEvent{
		EventMeta:  events.NewEventMeta(ctx),
		TaskID:     strconv.FormatUint(taskID, 10),
		NextIndex:  index,
		Generation: generation,
	}
	if err := event.HGValidate(); err != nil {
		return HGSendReadyEvent{}, err
	}
	return event, nil
}

// HGValidate 在消费边界再次校验事件的基础形态；数据库状态匹配仍由持久层验证。
func (event HGSendReadyEvent) HGValidate() error {
	id, err := strconv.ParseUint(event.TaskID, 10, 64)
	if err != nil || id == 0 || id > math.MaxInt64 || strconv.FormatUint(id, 10) != event.TaskID ||
		event.NextIndex >= 100 || event.Generation == 0 {
		return fmt.Errorf("通知发送事件参数无效")
	}
	return nil
}

// EventName 确保现有 Envelope 使用显式 v1 事件名，不复用业务关注通知事件。
func (event HGSendReadyEvent) EventName() string { return HGSendReadyEventName }

// EventKey 将同一任务的所有代次路由到同一分区；不在 Kafka key 中公开接收用户。
func (event HGSendReadyEvent) EventKey() string { return event.TaskID }

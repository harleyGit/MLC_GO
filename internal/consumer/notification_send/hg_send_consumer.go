package notification_send

import (
	"MLC_GO/internal/events"
	notificationevents "MLC_GO/internal/events/notification"
	hgkafka "MLC_GO/internal/pkg/kafka"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// HGConsumer 只用于专用 notification send topic；不得复用会忽略未知事件的关注通知消费组。
type HGConsumer struct {
	worker HGReadyDispatcher // 共享 MySQL 的当前代次租约与有界站内投递，不启动旧轮询循环。
}

// HGReadyDispatcher 只接受任务游标而非 Kafka 中的收件人快照。
type HGReadyDispatcher interface {
	HGDispatchReady(context.Context, notificationevents.HGSendReadyEvent) error
}

// HGNewConsumer 用注入的权威工作者处理任务事件；调用方负责消费者生命周期与 topic 鉴权。
func HGNewConsumer(worker HGReadyDispatcher) *HGConsumer { return &HGConsumer{worker: worker} }

// Handle 严格校验 Envelope 与最小 payload；失败写入 DLQ 后让任务由过期调度恢复，不盲发。
func (consumer *HGConsumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {
	if consumer == nil || consumer.worker == nil {
		return fmt.Errorf("通知 Kafka 消费依赖未就绪")
	}
	if envelope.EventName != notificationevents.HGSendReadyEventName || len(envelope.EventID) == 0 ||
		len(envelope.EventID) > 64 || envelope.Version != events.EventVersionV1 || len(envelope.Payload) > 512 {
		return hgkafka.HGNewTerminalError(fmt.Errorf("通知 Kafka 事件信封无效"))
	}
	var event notificationevents.HGSendReadyEvent
	decoder := json.NewDecoder(bytes.NewReader(envelope.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil || decoder.Decode(new(any)) != io.EOF ||
		event.HGValidate() != nil || event.TaskID != envelope.EventKey || event.Version != envelope.Version {
		return hgkafka.HGNewTerminalError(fmt.Errorf("通知 Kafka 事件坐标无效"))
	}
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return consumer.worker.HGDispatchReady(callCtx, event)
}

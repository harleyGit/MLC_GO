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

// PersistedEvent 是消费者交给 MySQL repository 的稳定内部模型。
type PersistedEvent struct {
	EventID        string
	EventName      string
	EventKey       string
	ActorUserID    string `json:"actorUserId"`
	SubmissionID   string `json:"submissionId"`
	Action         string `json:"action"`
	Active         bool   `json:"active"`
	Quantity       int    `json:"quantity"`
	FollowerID     string `json:"followerId"`
	FolloweeID     string `json:"followeeId"`
	KafkaTopic     string
	KafkaPartition int32
	KafkaOffset    int64
	Payload        string
}

type EventStore interface {
	ApplyEvent(context.Context, PersistedEvent) error
	ApplyEvents(context.Context, []PersistedEvent) error
}

type Consumer struct{ store EventStore }

func NewConsumer(store EventStore) *Consumer { return &Consumer{store: store} }

func (c *Consumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {
	event, interested, err := c.hgMapEvent(ctx, envelope)
	if err != nil || !interested {
		return err
	}
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

func (c *Consumer) hgMapEvent(ctx context.Context, envelope events.EventEnvelope) (PersistedEvent, bool, error) {
	if envelope.EventName != InteractionEventsPackage.VideoInteractionChangedEventName && envelope.EventName != InteractionEventsPackage.UserFollowChangedEventName {
		return PersistedEvent{}, false, nil
	}
	if c == nil || c.store == nil {
		return PersistedEvent{}, false, fmt.Errorf("interaction event store cannot be nil")
	}
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
	event.EventID, event.EventName, event.EventKey = envelope.EventID, envelope.EventName, envelope.EventKey
	event.KafkaTopic, event.KafkaPartition, event.KafkaOffset, event.Payload = delivery.Topic, delivery.Partition, delivery.Offset, string(envelope.Payload)
	return event, true, nil
}

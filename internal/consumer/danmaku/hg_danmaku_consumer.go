package danmaku

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	VideoDanmakuEventsPackage "MLC_GO/internal/events/video_danmaku"
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	"context"
	"encoding/json"
	"fmt"
)

// HistoryStore 按 Kafka 分区批量保存完整弹幕历史。
type HistoryStore interface {
	StoreDanmakuHistory(context.Context, []ClickHousePackage.HGDanmakuHistory) error
}

// RecentProjector 保存每个视频最近的有界弹幕集合。
type RecentProjector interface {
	Project(context.Context, consumer.Delivery, ClickHousePackage.HGDanmakuHistory) error
}

// Consumer 将专用 Kafka topic 投影到 ClickHouse 历史和 Redis 热数据。
type Consumer struct {
	store  HistoryStore
	recent RecentProjector
}

func NewConsumer(store HistoryStore, recent RecentProjector) *Consumer {
	return &Consumer{store: store, recent: recent}
}

func (c *Consumer) Handle(context.Context, events.EventEnvelope) error {
	return fmt.Errorf("danmaku consumer requires partition batch delivery")
}

// HandleBatch 先批量写 ClickHouse，再逐条推进 Redis offset 水位；任一步失败都不提交 Kafka offset。
func (c *Consumer) HandleBatch(ctx context.Context, delivered []consumer.DeliveredEnvelope) error {
	rows := make([]ClickHousePackage.HGDanmakuHistory, 0, len(delivered))
	deliveries := make([]consumer.Delivery, 0, len(delivered))
	for index, item := range delivered {
		if item.Envelope.EventName != VideoDanmakuEventsPackage.VideoDanmakuCreatedEventName {
			continue
		}
		var event VideoDanmakuEventsPackage.CreatedEvent
		if err := json.Unmarshal(item.Envelope.Payload, &event); err != nil {
			return fmt.Errorf("decode danmaku event at index %d: %w", index, err)
		}
		if event.DanmakuID == "" || event.VideoID == "" || event.UserID == "" || event.CreatedAt <= 0 {
			return fmt.Errorf("danmaku event at index %d is invalid", index)
		}
		rows = append(rows, ClickHousePackage.HGDanmakuHistory{
			DanmakuID: event.DanmakuID, SubmissionID: event.SubmissionID, VideoID: event.VideoID,
			UserID: event.UserID, RequestID: event.RequestID, Content: event.Content, ProgressMS: event.ProgressMS,
			Mode: event.Mode, Color: event.Color, FontSize: event.FontSize, CreatedAt: event.CreatedAt,
			KafkaTopic: item.Delivery.Topic, KafkaPartition: item.Delivery.Partition, KafkaOffset: item.Delivery.Offset,
		})
		deliveries = append(deliveries, item.Delivery)
	}
	if len(rows) == 0 {
		return nil
	}
	if c == nil || c.store == nil || c.recent == nil {
		return fmt.Errorf("danmaku consumer dependencies cannot be nil")
	}
	if err := c.store.StoreDanmakuHistory(ctx, rows); err != nil {
		return fmt.Errorf("store danmaku history: %w", err)
	}
	for index := range rows {
		if err := c.recent.Project(ctx, deliveries[index], rows[index]); err != nil {
			return fmt.Errorf("project recent danmaku: %w", err)
		}
	}
	return nil
}

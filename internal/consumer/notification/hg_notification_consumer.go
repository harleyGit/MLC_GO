package notification

import (
	"MLC_GO/internal/events"
	InteractionEventsPackage "MLC_GO/internal/events/interaction"
	NotificationPackage "MLC_GO/internal/modules/notification"
	"context"
	"encoding/json"
	"fmt"
)

// HGConsumer 只消费已提交的关系变更事件，写入成功才允许 Kafka 框架提交 offset。
// 站内通知以 event_id/receiver/category 唯一索引去重；重放不产生重复通知。
type HGConsumer struct {
	service *NotificationPackage.HGService // 复用站内信偏好与参数校验。
}

func HGNewConsumer(service *NotificationPackage.HGService) *HGConsumer {
	return &HGConsumer{service: service}
}

// Handle 忽略不属于本投影的事件；已识别但不合法的事件必须返回错误，不能丢弃 offset。
func (c *HGConsumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {
	if envelope.EventName != InteractionEventsPackage.HGUserFollowCommittedEventName {
		return nil
	}
	if c == nil || c.service == nil || len(envelope.EventID) == 0 || len(envelope.EventID) > 64 {
		return fmt.Errorf("通知事件 ID 或服务未就绪")
	}
	var event InteractionEventsPackage.HGUserFollowCommittedEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return fmt.Errorf("解码已提交关注事件: %w", err)
	}
	if event.FolloweeID == "" || event.FollowerID == "" || event.FollowerID == event.FolloweeID || event.FollowID == "" || envelope.EventKey != event.FolloweeID {
		return NotificationPackage.HGErrInvalid
	}
	return c.service.HGDeliver(ctx, envelope.EventID, event.FolloweeID, NotificationPackage.HGNotice{
		Category: "follow", Title: "新增粉丝", Content: "有人关注了你", TargetType: "user", TargetID: event.FollowerID,
	})
}

package im

import (
	"MLC_GO/internal/events"
	IM "MLC_GO/internal/modules/im"
	"context"
)

// HGConsumer 独立 IM 消费组，只在 Inbox 与逐设备任务同事务落库后返回成功。
type HGConsumer struct {
	service *IM.HGRealtime /* IM 权威投递服务，不能为 nil。 */
}

func HGNewConsumer(service *IM.HGRealtime) *HGConsumer { return &HGConsumer{service: service} }
func (c *HGConsumer) Handle(ctx context.Context, e events.EventEnvelope) error {
	return c.service.HGIngest(ctx, e)
}

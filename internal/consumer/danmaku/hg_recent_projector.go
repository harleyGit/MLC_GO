package danmaku

import (
	"MLC_GO/internal/consumer"
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"fmt"
)

type hgRedisEvalClient interface {
	Eval(context.Context, string, []string, ...any) error
}

type HGRecentProjector struct {
	redis *PersistenceRedisPackage.RedisService
	limit int
}

func NewRecentProjector(redis *PersistenceRedisPackage.RedisService, limit int) *HGRecentProjector {
	return &HGRecentProjector{redis: redis, limit: limit}
}

func (p *HGRecentProjector) Project(ctx context.Context, delivery consumer.Delivery, item ClickHousePackage.HGDanmakuHistory) error {
	if p == nil || p.redis == nil || p.limit <= 0 {
		return fmt.Errorf("danmaku recent projector is invalid")
	}
	keys := []string{PersistenceRedisPackage.GetVideoDanmakuRecentStreamKey(item.VideoID), PersistenceRedisPackage.GetVideoDanmakuRecentOffsetKey(item.VideoID)}
	watermark := fmt.Sprintf("%s:%d", delivery.Topic, delivery.Partition)
	return p.redis.Eval(ctx, PersistenceRedisPackage.VideoDanmakuRecentProjectLuaScript, keys,
		watermark, delivery.Offset, p.limit, item.DanmakuID, item.SubmissionID, item.VideoID, item.Content,
		item.ProgressMS, item.Mode, item.Color, item.FontSize, item.CreatedAt)
}

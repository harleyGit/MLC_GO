package VideoDanmakuRealtimePackage

import (
	VideoDanmakuDtoPackage "MLC_GO/internal/modules/video_danmaku/dto"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

type hgRoomSubscriptionCommand struct {
	channel     string
	unsubscribe bool
}

// hgRoomRouter 只订阅本实例存在连接的房间，避免所有网关接收全平台广播。
type hgRoomRouter struct {
	redis     *PersistenceRedisPackage.RedisService
	pubsub    *redis.PubSub
	refsMu    sync.Mutex
	refs      map[string]int
	commands  chan hgRoomSubscriptionCommand
	onMessage func(VideoDanmakuDtoPackage.DanmakuResponse)
	closeOnce sync.Once
}

func newHGRoomRouter(redisService *PersistenceRedisPackage.RedisService, queueSize int, onMessage func(VideoDanmakuDtoPackage.DanmakuResponse)) *hgRoomRouter {
	if queueSize < 64 {
		queueSize = 64
	}
	router := &hgRoomRouter{redis: redisService, refs: make(map[string]int), commands: make(chan hgRoomSubscriptionCommand, queueSize), onMessage: onMessage}
	if redisService != nil && redisService.Client() != nil {
		router.pubsub = redisService.Client().Subscribe(context.Background())
	}
	return router
}

// Join 增加本地房间引用；首个连接必须确认 Redis 订阅成功后才完成 WebSocket 握手。
func (r *hgRoomRouter) Join(ctx context.Context, videoID string) error {
	if r == nil || r.redis == nil || r.redis.Client() == nil || r.pubsub == nil {
		return fmt.Errorf("danmaku room router redis cannot be nil")
	}
	r.refsMu.Lock()
	defer r.refsMu.Unlock()
	if r.refs[videoID] > 0 {
		r.refs[videoID]++
		return nil
	}
	if err := r.pubsub.Subscribe(ctx, PersistenceRedisPackage.GetVideoDanmakuBroadcastChannel(videoID)); err != nil {
		return fmt.Errorf("subscribe danmaku room: %w", err)
	}
	r.refs[videoID] = 1
	return nil
}

// Leave 仅在最后一个本地连接离开时异步退订，event-loop 不执行 Redis I/O。
func (r *hgRoomRouter) Leave(videoID string) {
	if r == nil || videoID == "" {
		return
	}
	r.refsMu.Lock()
	count := r.refs[videoID]
	if count > 1 {
		r.refs[videoID] = count - 1
		r.refsMu.Unlock()
		return
	}
	delete(r.refs, videoID)
	r.refsMu.Unlock()
	select {
	case r.commands <- hgRoomSubscriptionCommand{channel: PersistenceRedisPackage.GetVideoDanmakuBroadcastChannel(videoID), unsubscribe: true}:
	default:
		// 队列只承载“最后连接离开”事件；满时保留多余订阅比阻塞 event-loop 更安全。
	}
}

func (r *hgRoomRouter) Run(ctx context.Context) error {
	if r == nil || r.redis == nil || r.redis.Client() == nil {
		return fmt.Errorf("danmaku room router redis cannot be nil")
	}
	if r.pubsub == nil {
		r.pubsub = r.redis.Client().Subscribe(ctx)
	}
	defer r.pubsub.Close()
	messages := r.pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case command := <-r.commands:
			if command.unsubscribe {
				_ = r.pubsub.Unsubscribe(ctx, command.channel)
			}
		case message, ok := <-messages:
			if !ok {
				return fmt.Errorf("danmaku room subscription closed")
			}
			var item VideoDanmakuDtoPackage.DanmakuResponse
			if json.Unmarshal([]byte(message.Payload), &item) == nil && item.VideoID != "" && r.onMessage != nil {
				r.onMessage(item)
			}
		}
	}
}

func (r *hgRoomRouter) Publish(ctx context.Context, item VideoDanmakuDtoPackage.DanmakuResponse) error {
	payload, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return r.redis.Client().Publish(ctx, PersistenceRedisPackage.GetVideoDanmakuBroadcastChannel(item.VideoID), payload).Err()
}

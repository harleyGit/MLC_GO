package VideoDanmakuRealtimePackage

import (
	VideoDanmakuDtoPackage "MLC_GO/internal/modules/video_danmaku/dto"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
)

type hgRoomSubscriptionCommand struct {
	videoID    string
	generation uint64
}

type hgRoomSubscriptionState struct {
	refs        int
	generation  uint64
	subscribed  bool
	subscribing bool
	ready       chan struct{}
	err         error
}

// hgRoomRouter 只订阅本实例存在连接的房间，避免所有网关接收全平台广播。
type hgRoomRouter struct {
	redis     *PersistenceRedisPackage.RedisService
	pubsub    *redis.PubSub
	refsMu    sync.Mutex
	refs      map[string]*hgRoomSubscriptionState
	commands  chan hgRoomSubscriptionCommand
	onMessage func(VideoDanmakuDtoPackage.DanmakuResponse)
	running   atomic.Bool
	runErrMu  sync.RWMutex
	runErr    error
}

func newHGRoomRouter(redisService *PersistenceRedisPackage.RedisService, queueSize int, onMessage func(VideoDanmakuDtoPackage.DanmakuResponse)) *hgRoomRouter {
	if queueSize < 64 {
		queueSize = 64
	}
	router := &hgRoomRouter{redis: redisService, refs: make(map[string]*hgRoomSubscriptionState), commands: make(chan hgRoomSubscriptionCommand, queueSize), onMessage: onMessage}
	if redisService != nil && redisService.Client() != nil {
		router.pubsub = redisService.Client().Subscribe(context.Background())
	}
	return router
}

// Join 增加本地房间引用；Redis I/O 不持有全局引用锁，同一房间的并发首连共享订阅结果。
func (r *hgRoomRouter) Join(ctx context.Context, videoID string) error {
	if r == nil || r.redis == nil || r.redis.Client() == nil || r.pubsub == nil {
		return fmt.Errorf("danmaku room router redis cannot be nil")
	}
	for {
		r.refsMu.Lock()
		state := r.refs[videoID]
		if state == nil {
			state = &hgRoomSubscriptionState{}
			r.refs[videoID] = state
		}
		if state.subscribed {
			if state.refs == 0 {
				state.generation++
			}
			state.refs++
			r.refsMu.Unlock()
			return nil
		}
		if state.subscribing {
			ready, generation := state.ready, state.generation
			r.refsMu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ready:
			}
			r.refsMu.Lock()
			state = r.refs[videoID]
			if state != nil && state.generation == generation && state.err != nil {
				err := state.err
				r.refsMu.Unlock()
				return err
			}
			r.refsMu.Unlock()
			continue
		}
		state.generation++
		generation := state.generation
		state.subscribing = true
		state.ready = make(chan struct{})
		state.err = nil
		r.refsMu.Unlock()

		err := r.pubsub.Subscribe(ctx, PersistenceRedisPackage.GetVideoDanmakuBroadcastChannel(videoID))
		r.refsMu.Lock()
		state = r.refs[videoID]
		if state == nil || state.generation != generation {
			r.refsMu.Unlock()
			if err == nil {
				_ = r.pubsub.Unsubscribe(context.Background(), PersistenceRedisPackage.GetVideoDanmakuBroadcastChannel(videoID))
			}
			continue
		}
		state.subscribing = false
		state.err = err
		if err == nil {
			state.subscribed = true
			state.refs++
		}
		close(state.ready)
		r.refsMu.Unlock()
		if err != nil {
			return fmt.Errorf("subscribe danmaku room: %w", err)
		}
		return nil
	}
}

// Leave 仅在最后一个本地连接离开时异步退订，generation 防止旧退订移除已重新加入的房间。
func (r *hgRoomRouter) Leave(videoID string) {
	if r == nil || videoID == "" {
		return
	}
	r.refsMu.Lock()
	state := r.refs[videoID]
	if state == nil || state.refs == 0 {
		r.refsMu.Unlock()
		return
	}
	state.refs--
	if state.refs > 0 {
		r.refsMu.Unlock()
		return
	}
	state.generation++
	command := hgRoomSubscriptionCommand{videoID: videoID, generation: state.generation}
	r.refsMu.Unlock()
	select {
	case r.commands <- command:
	default:
		// 满队列时保留多余订阅比阻塞 event-loop 更安全，后续加入仍会复用该订阅。
	}
}

func (r *hgRoomRouter) Run(ctx context.Context) error {
	if r == nil || r.redis == nil || r.redis.Client() == nil {
		return fmt.Errorf("danmaku room router redis cannot be nil")
	}
	if r.pubsub == nil {
		r.pubsub = r.redis.Client().Subscribe(ctx)
	}
	r.running.Store(true)
	r.hgSetRunError(nil)
	defer func() {
		r.running.Store(false)
		_ = r.pubsub.Close()
	}()
	messages := r.pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case command := <-r.commands:
			if !r.hgShouldUnsubscribe(command) {
				continue
			}
			if err := r.pubsub.Unsubscribe(ctx, PersistenceRedisPackage.GetVideoDanmakuBroadcastChannel(command.videoID)); err != nil {
				r.hgSetRunError(fmt.Errorf("unsubscribe danmaku room: %w", err))
				return r.hgRunError()
			}
			r.hgMarkUnsubscribed(command)
		case message, ok := <-messages:
			if !ok {
				err := fmt.Errorf("danmaku room subscription closed")
				r.hgSetRunError(err)
				return err
			}
			var item VideoDanmakuDtoPackage.DanmakuResponse
			if json.Unmarshal([]byte(message.Payload), &item) == nil && item.VideoID != "" && r.onMessage != nil {
				r.onMessage(item)
			}
		}
	}
}

func (r *hgRoomRouter) Ready() error {
	if r == nil || !r.running.Load() {
		if err := r.hgRunError(); err != nil {
			return err
		}
		return fmt.Errorf("danmaku room router not ready")
	}
	return nil
}

func (r *hgRoomRouter) hgShouldUnsubscribe(command hgRoomSubscriptionCommand) bool {
	r.refsMu.Lock()
	defer r.refsMu.Unlock()
	state := r.refs[command.videoID]
	return state != nil && state.generation == command.generation && state.refs == 0 && state.subscribed && !state.subscribing
}

func (r *hgRoomRouter) hgMarkUnsubscribed(command hgRoomSubscriptionCommand) {
	r.refsMu.Lock()
	defer r.refsMu.Unlock()
	state := r.refs[command.videoID]
	if state != nil && state.generation == command.generation && state.refs == 0 {
		delete(r.refs, command.videoID)
	}
}

func (r *hgRoomRouter) hgSetRunError(err error) {
	r.runErrMu.Lock()
	r.runErr = err
	r.runErrMu.Unlock()
}

func (r *hgRoomRouter) hgRunError() error {
	r.runErrMu.RLock()
	defer r.runErrMu.RUnlock()
	return r.runErr
}

func (r *hgRoomRouter) Publish(ctx context.Context, item VideoDanmakuDtoPackage.DanmakuResponse) error {
	if r == nil || r.redis == nil || r.redis.Client() == nil {
		return fmt.Errorf("danmaku room router redis cannot be nil")
	}
	payload, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return r.redis.Client().Publish(ctx, PersistenceRedisPackage.GetVideoDanmakuBroadcastChannel(item.VideoID), payload).Err()
}

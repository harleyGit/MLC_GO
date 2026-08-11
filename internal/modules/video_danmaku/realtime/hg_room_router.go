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
	refs          int
	generation    uint64
	subscribed    bool
	subscribing   bool
	unsubscribing bool
	ready         chan struct{}
	err           error
}

// hgRoomRouter 只订阅本实例存在连接的房间，避免所有网关接收全平台广播。
type hgRoomRouter struct {
	redis     *PersistenceRedisPackage.RedisService
	pubsub    *redis.PubSub
	refsMu    sync.Mutex
	refs      map[string]*hgRoomSubscriptionState
	pendingMu sync.Mutex
	pending   map[string]hgRoomSubscriptionCommand
	wake      chan struct{}
	onMessage func(VideoDanmakuDtoPackage.DanmakuResponse)
	running   atomic.Bool
	runErrMu  sync.RWMutex
	runErr    error
}

func newHGRoomRouter(redisService *PersistenceRedisPackage.RedisService, queueSize int, onMessage func(VideoDanmakuDtoPackage.DanmakuResponse)) *hgRoomRouter {
	if queueSize < 64 {
		queueSize = 64
	}
	router := &hgRoomRouter{
		redis:     redisService,
		refs:      make(map[string]*hgRoomSubscriptionState),
		pending:   make(map[string]hgRoomSubscriptionCommand, queueSize),
		wake:      make(chan struct{}, 1),
		onMessage: onMessage,
	}
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
		if state.unsubscribing {
			ready := state.ready
			r.refsMu.Unlock()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ready:
			}
			r.refsMu.Lock()
			state = r.refs[videoID]
			if state != nil && state.err != nil {
				err := state.err
				r.refsMu.Unlock()
				return err
			}
			r.refsMu.Unlock()
			continue
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
	r.hgQueueUnsubscribe(command)
}

// hgQueueUnsubscribe 按房间只保留最新代次，并用容量为 1 的信号唤醒 Run。待处理集合的
// 上限受本实例房间数约束，不会因为瞬时退房超过 channel 容量而永久泄漏 Redis 订阅。
func (r *hgRoomRouter) hgQueueUnsubscribe(command hgRoomSubscriptionCommand) {
	r.pendingMu.Lock()
	r.pending[command.videoID] = command
	r.pendingMu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *hgRoomRouter) hgPopUnsubscribe() (hgRoomSubscriptionCommand, bool) {
	r.pendingMu.Lock()
	var command hgRoomSubscriptionCommand
	for videoID, pending := range r.pending {
		command = pending
		delete(r.pending, videoID)
		break
	}
	hasMore := len(r.pending) > 0
	r.pendingMu.Unlock()
	if hasMore {
		select {
		case r.wake <- struct{}{}:
		default:
		}
	}
	return command, command.videoID != ""
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
		case <-r.wake:
			command, ok := r.hgPopUnsubscribe()
			if !ok || !r.hgBeginUnsubscribe(command) {
				continue
			}
			err := r.pubsub.Unsubscribe(ctx, PersistenceRedisPackage.GetVideoDanmakuBroadcastChannel(command.videoID))
			if err != nil {
				err = fmt.Errorf("unsubscribe danmaku room: %w", err)
			}
			r.hgFinishUnsubscribe(command, err)
			if err != nil {
				r.hgSetRunError(err)
				return r.hgRunError()
			}
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

// hgBeginUnsubscribe 在 Redis I/O 前原子标记退订中。此后到达的 Join 必须等待退订结束，
// 不能再把即将被取消的底层订阅误认为可复用，从而避免“校验通过后立即重连”的竞态。
func (r *hgRoomRouter) hgBeginUnsubscribe(command hgRoomSubscriptionCommand) bool {
	r.refsMu.Lock()
	defer r.refsMu.Unlock()
	state := r.refs[command.videoID]
	if state == nil || state.generation != command.generation || state.refs != 0 || !state.subscribed || state.subscribing || state.unsubscribing {
		return false
	}
	state.unsubscribing = true
	state.ready = make(chan struct{})
	state.err = nil
	return true
}

func (r *hgRoomRouter) hgFinishUnsubscribe(command hgRoomSubscriptionCommand, unsubscribeErr error) {
	r.refsMu.Lock()
	defer r.refsMu.Unlock()
	state := r.refs[command.videoID]
	if state == nil || state.generation != command.generation || !state.unsubscribing {
		return
	}
	state.unsubscribing = false
	state.err = unsubscribeErr
	if unsubscribeErr == nil {
		state.subscribed = false
	}
	close(state.ready)
	if unsubscribeErr == nil && state.refs == 0 {
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

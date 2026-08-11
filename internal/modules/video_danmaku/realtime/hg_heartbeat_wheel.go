package VideoDanmakuRealtimePackage

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/gnet/v2"
)

const hgHeartbeatWheelTick = time.Second

type hgHeartbeatWheelShard struct {
	mu      sync.Mutex
	cursor  uint64
	buckets []map[*hgConnection]struct{}
}

// hgHeartbeatWheel 将连接分散到固定数量的时间槽和锁分片。连接流量只更新 lastActive，
// 不移动时间轮节点，因此入站热路径不会因每个 Ping、Pong 或弹幕帧增加锁竞争和容器操作。
type hgHeartbeatWheel struct {
	tick      time.Duration
	interval  time.Duration
	timeout   time.Duration
	shards    []hgHeartbeatWheelShard
	nextShard atomic.Uint32
	sequence  atomic.Uint64
}

func newHGHeartbeatWheel(shardCount int, interval, timeout time.Duration) *hgHeartbeatWheel {
	if shardCount <= 0 || shardCount&(shardCount-1) != 0 {
		shardCount = 4
	}
	if interval <= 0 {
		interval = 20 * time.Second
	}
	if timeout < 2*interval {
		timeout = 3 * interval
	}
	// 加一个槽可避免最大 timeout 恰好整圈时与当前槽重合；bucket map 按需创建，
	// 初始化成本仅与分片数和槽指针数相关，不与最大连接数相关。
	slotCount := hgHeartbeatDurationTicks(timeout, hgHeartbeatWheelTick) + 1
	wheel := &hgHeartbeatWheel{
		tick:     hgHeartbeatWheelTick,
		interval: interval,
		timeout:  timeout,
		shards:   make([]hgHeartbeatWheelShard, shardCount),
	}
	for index := range wheel.shards {
		wheel.shards[index].buckets = make([]map[*hgConnection]struct{}, slotCount)
	}
	return wheel
}

// hgRegister 只在 WebSocket 完成鉴权和 Upgrade 后调用。首次到期时间在一个心跳周期内
// 确定性打散，避免滚动发布或网络恢复后的海量重连在同一秒集中发送 Ping。
func (w *hgHeartbeatWheel) hgRegister(state *hgConnection, conn gnet.Conn) {
	if w == nil || state == nil || conn == nil || state.heartbeatClosed.Load() {
		return
	}
	state.heartbeatConn = conn
	w.hgRegisterState(state)
}

func (w *hgHeartbeatWheel) hgRegisterState(state *hgConnection) {
	if w == nil || state == nil || state.heartbeatClosed.Load() {
		return
	}
	state.heartbeatShard = (w.nextShard.Add(1) - 1) & uint32(len(w.shards)-1)
	intervalTicks := hgHeartbeatDurationTicks(w.interval, w.tick)
	delayTicks := uint64(1)
	if intervalTicks > 1 {
		delayTicks += (w.sequence.Add(1) - 1) % uint64(intervalTicks)
	}
	w.hgScheduleTicks(state, delayTicks)
}

// hgCancel 可由 OnClose 重复调用。关闭标志必须先于 bucket 删除生效，因为节点可能已经被
// tick 线程取到本地切片；该标志会阻止取出后的节点再次注册到未来时间槽。
func (w *hgHeartbeatWheel) hgCancel(state *hgConnection) {
	if w == nil || state == nil {
		return
	}
	state.heartbeatClosed.Store(true)
	if int(state.heartbeatShard) >= len(w.shards) {
		return
	}
	shard := &w.shards[state.heartbeatShard]
	shard.mu.Lock()
	if state.heartbeatScheduled && int(state.heartbeatSlot) < len(shard.buckets) {
		delete(shard.buckets[state.heartbeatSlot], state)
		state.heartbeatScheduled = false
	}
	shard.mu.Unlock()
}

// hgAdvance 每次仅取出各分片当前时间槽，而不是扫描全部房间和连接。返回后调用方才能执行
// AsyncWrite 或 Close，确保网络操作不会延长时间轮锁的临界区。
func (w *hgHeartbeatWheel) hgAdvance() []*hgConnection {
	if w == nil {
		return nil
	}
	var due []*hgConnection
	for index := range w.shards {
		shard := &w.shards[index]
		shard.mu.Lock()
		shard.cursor++
		slot := int(shard.cursor % uint64(len(shard.buckets)))
		bucket := shard.buckets[slot]
		shard.buckets[slot] = nil
		for state := range bucket {
			state.heartbeatScheduled = false
			due = append(due, state)
		}
		shard.mu.Unlock()
	}
	return due
}

// hgReschedule 根据最近活动时间选择下一次检查点：活跃连接按 interval 发送 Ping，静默连接
// 最迟在 timeout 附近再次检查。OnTraffic 无需触碰时间轮锁。
func (w *hgHeartbeatWheel) hgReschedule(state *hgConnection, now time.Time) bool {
	if w == nil || state == nil || state.heartbeatClosed.Load() {
		return false
	}
	remaining := time.Duration(state.lastActive.Load()-now.UnixNano()) + w.timeout
	if remaining <= 0 {
		return false
	}
	delay := min(w.interval, remaining)
	w.hgScheduleTicks(state, uint64(hgHeartbeatDurationTicks(delay, w.tick)))
	return !state.heartbeatClosed.Load()
}

func (w *hgHeartbeatWheel) hgScheduleTicks(state *hgConnection, delayTicks uint64) {
	if delayTicks == 0 {
		delayTicks = 1
	}
	shard := &w.shards[state.heartbeatShard]
	shard.mu.Lock()
	defer shard.mu.Unlock()
	if state.heartbeatClosed.Load() {
		return
	}
	if state.heartbeatScheduled && int(state.heartbeatSlot) < len(shard.buckets) {
		delete(shard.buckets[state.heartbeatSlot], state)
	}
	slot := uint32((shard.cursor + delayTicks) % uint64(len(shard.buckets)))
	if shard.buckets[slot] == nil {
		shard.buckets[slot] = make(map[*hgConnection]struct{})
	}
	shard.buckets[slot][state] = struct{}{}
	state.heartbeatSlot = slot
	state.heartbeatScheduled = true
}

func hgHeartbeatDurationTicks(duration, tick time.Duration) int {
	if duration <= 0 {
		return 1
	}
	return int((duration + tick - 1) / tick)
}

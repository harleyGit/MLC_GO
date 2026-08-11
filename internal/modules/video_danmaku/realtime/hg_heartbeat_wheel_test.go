package VideoDanmakuRealtimePackage

import (
	"sync"
	"testing"
	"time"
)

func TestHeartbeatWheelSpreadsInitialScheduleAcrossShardsAndSlots(t *testing.T) {
	wheel := newHGHeartbeatWheel(4, 4*time.Second, 10*time.Second)
	states := make([]*hgConnection, 8)
	seenShards := make(map[uint32]struct{})
	seenSlots := make(map[uint32]struct{})
	for index := range states {
		states[index] = &hgConnection{}
		wheel.hgRegisterState(states[index])
		seenShards[states[index].heartbeatShard] = struct{}{}
		seenSlots[states[index].heartbeatSlot] = struct{}{}
	}
	if len(seenShards) != 4 {
		t.Fatalf("heartbeat shards = %d, want 4", len(seenShards))
	}
	if len(seenSlots) < 4 {
		t.Fatalf("heartbeat slots = %d, want at least 4", len(seenSlots))
	}
}

func TestHeartbeatWheelCancelPreventsDueAndReschedule(t *testing.T) {
	wheel := newHGHeartbeatWheel(4, time.Second, 3*time.Second)
	state := &hgConnection{}
	wheel.hgRegisterState(state)
	wheel.hgCancel(state)
	wheel.hgCancel(state)

	if due := wheel.hgAdvance(); len(due) != 0 {
		t.Fatalf("cancelled heartbeat due count = %d, want 0", len(due))
	}
	state.lastActive.Store(time.Now().UnixNano())
	if wheel.hgReschedule(state, time.Now()) {
		t.Fatal("closed heartbeat was rescheduled")
	}
}

func TestHeartbeatWheelActiveConnectionIsRescheduled(t *testing.T) {
	wheel := newHGHeartbeatWheel(4, time.Second, 3*time.Second)
	state := &hgConnection{}
	state.lastActive.Store(time.Now().UnixNano())
	wheel.hgRegisterState(state)
	due := wheel.hgAdvance()
	if len(due) != 1 || due[0] != state {
		t.Fatalf("heartbeat due = %v, want registered state", due)
	}
	if !wheel.hgReschedule(state, time.Now()) || !state.heartbeatScheduled {
		t.Fatal("active heartbeat was not rescheduled")
	}
}

func TestHeartbeatWheelConcurrentCancelAndReschedule(t *testing.T) {
	wheel := newHGHeartbeatWheel(4, time.Second, 3*time.Second)
	state := &hgConnection{}
	state.lastActive.Store(time.Now().UnixNano())
	wheel.hgRegisterState(state)
	_ = wheel.hgAdvance()

	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		wheel.hgCancel(state)
	}()
	go func() {
		defer group.Done()
		wheel.hgReschedule(state, time.Now())
	}()
	group.Wait()
	if !state.heartbeatClosed.Load() {
		t.Fatal("heartbeat connection was not marked closed")
	}

	shard := &wheel.shards[state.heartbeatShard]
	shard.mu.Lock()
	scheduled := state.heartbeatScheduled
	shard.mu.Unlock()
	if scheduled {
		t.Fatal("closed heartbeat remained scheduled")
	}
}

func BenchmarkHeartbeatWheelScheduleAndAdvance100K(b *testing.B) {
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		wheel := newHGHeartbeatWheel(64, 20*time.Second, 60*time.Second)
		states := make([]hgConnection, 100_000)
		for index := range states {
			wheel.hgRegisterState(&states[index])
		}
		for tick := 0; tick < 20; tick++ {
			_ = wheel.hgAdvance()
		}
	}
}

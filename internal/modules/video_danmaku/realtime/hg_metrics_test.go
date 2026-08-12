package VideoDanmakuRealtimePackage

import (
	VideoDanmakuDtoPackage "MLC_GO/internal/modules/video_danmaku/dto"
	ConfigPackage "MLC_GO/internal/pkg/config"
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/panjf2000/gnet/v2"
)

type hgMetricsTestConn struct {
	gnet.Conn
	state         *hgConnection
	asyncWriteErr error
	callbackErr   error
	writes        atomic.Int64
	closes        atomic.Int64
}

func (c *hgMetricsTestConn) Context() any { return c.state }

func (c *hgMetricsTestConn) SetContext(value any) {
	c.state, _ = value.(*hgConnection)
}

func (c *hgMetricsTestConn) AsyncWrite(_ []byte, callback gnet.AsyncCallback) error {
	c.writes.Add(1)
	if c.asyncWriteErr != nil {
		return c.asyncWriteErr
	}
	if callback != nil {
		return callback(c, c.callbackErr)
	}
	return nil
}

func (c *hgMetricsTestConn) Close() error { c.closes.Add(1); return nil }

func TestRealtimeMetricsExposeFixedCardinalitySnapshots(t *testing.T) {
	server := NewServer(nil, nil, ConfigPackage.HGVideoDanmakuConfig{
		QueueSize: 4, MaxConnections: 100, RoomShardCount: 16, MemberShardCount: 4,
		BroadcastWorkerCount: 2, BroadcastQueueSize: 4,
	})
	server.metrics.websocketConnections.Store(3)
	server.metrics.activeRooms.Store(2)
	server.metrics.outboundPendingBytes.Store(128)
	server.metrics.commandsReceived.Store(10)
	server.metrics.commandRejections[hgCommandRejectionRateLimited].Store(2)
	server.metrics.publishResults[hgPublishSuccess].Store(7)
	server.metrics.recipientWrites[hgRecipientQueued].Store(9)
	server.metrics.outboundFailures[hgOutboundFailurePendingBudget].Store(1)
	server.metrics.hgObserveBroadcastDuration(7 * time.Millisecond)
	server.lifecycle.Store(int32(hgLifecycleDraining))
	server.metrics.drainStarts.Store(1)
	server.metrics.lateHandshakeRejections.Store(2)
	server.queue <- hgCommand{}
	server.broadcastQueue[0] <- VideoDanmakuDtoPackage.DanmakuResponse{VideoID: "video-secret"}

	var output bytes.Buffer
	server.HGWritePrometheusMetrics(&output)
	metrics := output.String()
	for _, expected := range []string{
		"mlc_video_danmaku_websocket_connections 3",
		"mlc_video_danmaku_command_queue_size 1",
		"mlc_video_danmaku_broadcast_queue_size 1",
		`mlc_video_danmaku_command_rejections_total{reason="rate_limited"} 2`,
		`mlc_video_danmaku_broadcast_publish_total{result="success"} 7`,
		`mlc_video_danmaku_broadcast_recipient_writes_total{result="queued"} 9`,
		`mlc_video_danmaku_outbound_write_failures_total{reason="pending_budget"} 1`,
		`mlc_video_danmaku_broadcast_duration_seconds_bucket{le="0.01"} 1`,
		"mlc_video_danmaku_broadcast_duration_seconds_count 1",
		"mlc_video_danmaku_lifecycle_state 2",
		"mlc_video_danmaku_drain_starts_total 1",
		"mlc_video_danmaku_drain_late_handshake_rejections_total 2",
	} {
		if !strings.Contains(metrics, expected) {
			t.Fatalf("metrics missing %q:\n%s", expected, metrics)
		}
	}
	for _, forbidden := range []string{"video-secret", "video_id", "user_id", "request_id", "error_message"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics contain forbidden high-cardinality value %q", forbidden)
		}
	}
}

func TestBroadcastDurationHistogramIsCumulativeAndConcurrent(t *testing.T) {
	server := &Server{}
	const observations = 1_000
	var workers sync.WaitGroup
	workers.Add(observations)
	for index := 0; index < observations; index++ {
		go func(value int) {
			defer workers.Done()
			server.metrics.hgObserveBroadcastDuration(time.Duration(value%20+1) * time.Millisecond)
		}(index)
	}
	workers.Wait()

	var output bytes.Buffer
	server.HGWritePrometheusMetrics(&output)
	metrics := output.String()
	if !strings.Contains(metrics, "mlc_video_danmaku_broadcast_duration_seconds_count 1000") ||
		!strings.Contains(metrics, `mlc_video_danmaku_broadcast_duration_seconds_bucket{le="+Inf"} 1000`) {
		t.Fatalf("histogram count mismatch:\n%s", metrics)
	}
}

func TestBroadcastQueueDropIsCountedWithoutBlocking(t *testing.T) {
	server := NewServer(nil, nil, ConfigPackage.HGVideoDanmakuConfig{RoomShardCount: 16, MemberShardCount: 4, BroadcastWorkerCount: 1, BroadcastQueueSize: 1})
	item := VideoDanmakuDtoPackage.DanmakuResponse{VideoID: "video-1"}
	server.hgEnqueueBroadcast(item)
	server.hgEnqueueBroadcast(item)
	if got := server.metrics.broadcastQueueDropped.Load(); got != 1 {
		t.Fatalf("broadcast queue drops = %d, want 1", got)
	}
}

func TestWebSocketConnectionGaugeIsReleasedOnce(t *testing.T) {
	server := &Server{}
	state := &hgConnection{}
	server.hgCountWebSocketConnection(state)
	server.hgCountWebSocketConnection(state)
	server.hgReleaseWebSocketConnection(state)
	server.hgReleaseWebSocketConnection(state)
	if got := server.metrics.websocketConnections.Load(); got != 0 {
		t.Fatalf("websocket connections = %d, want 0", got)
	}
}

func TestClosedConnectionCannotBeActivatedByLateUpgrade(t *testing.T) {
	server := NewServer(nil, nil, ConfigPackage.HGVideoDanmakuConfig{
		RoomShardCount: 16, MemberShardCount: 4, BroadcastWorkerCount: 1, BroadcastQueueSize: 1,
		HeartbeatShardCount: 4, HeartbeatInterval: time.Second, HeartbeatTimeout: 2 * time.Second,
	})
	state := &hgConnection{}
	state.closed.Store(true)
	conn := &hgMetricsTestConn{state: state}
	if server.hgActivateWebSocket("video-1", "user-1", conn, state) {
		t.Fatal("closed connection was activated by late upgrade")
	}
	if got := server.metrics.websocketConnections.Load(); got != 0 {
		t.Fatalf("websocket connections = %d, want 0", got)
	}
	if got := server.metrics.activeRooms.Load(); got != 0 {
		t.Fatalf("active rooms = %d, want 0", got)
	}
	if state.videoID != "" || state.upgraded {
		t.Fatal("closed connection retained upgraded state")
	}
}

func TestBeginDrainIsIdempotentAndKeepsExistingConnectionOpen(t *testing.T) {
	server := NewServer(nil, nil, ConfigPackage.HGVideoDanmakuConfig{
		RoomShardCount: 16, MemberShardCount: 4, BroadcastWorkerCount: 1, BroadcastQueueSize: 1,
		HeartbeatShardCount: 4, HeartbeatInterval: time.Second, HeartbeatTimeout: 2 * time.Second,
		MaxPendingBytes: 1024,
	})
	server.lifecycle.Store(int32(hgLifecycleServing))
	state := &hgConnection{videoID: "video-1", userID: "user-1", upgraded: true}
	server.hgCountWebSocketConnection(state)
	conn := &hgMetricsTestConn{state: state}
	server.hgJoin("video-1", conn, state)

	server.BeginDrain()
	server.BeginDrain()
	if got := hgLifecycleState(server.lifecycle.Load()); got != hgLifecycleDraining {
		t.Fatalf("lifecycle = %d, want draining", got)
	}
	if got := server.metrics.drainStarts.Load(); got != 1 {
		t.Fatalf("drain starts = %d, want 1", got)
	}
	if got := conn.writes.Load(); got != 1 {
		t.Fatalf("drain close frames = %d, want 1", got)
	}
	if got := conn.closes.Load(); got != 0 {
		t.Fatalf("connection closed immediately during drain: %d", got)
	}
	if err := server.hgWriteDataFrame(conn, state, []byte("business frame")); !errors.Is(err, hgErrConnectionDraining) {
		t.Fatalf("business frame after Close error = %v, want draining", err)
	}
	if got := conn.writes.Load(); got != 1 {
		t.Fatalf("business data was queued after Close Frame: writes=%d", got)
	}
	if err := server.Ready(); err == nil {
		t.Fatal("draining server remained ready")
	}
}

func TestDrainingServerRejectsLateUpgradeActivation(t *testing.T) {
	server := NewServer(nil, nil, ConfigPackage.HGVideoDanmakuConfig{
		RoomShardCount: 16, MemberShardCount: 4, BroadcastWorkerCount: 1, BroadcastQueueSize: 1,
		HeartbeatShardCount: 4, HeartbeatInterval: time.Second, HeartbeatTimeout: 2 * time.Second,
	})
	server.lifecycle.Store(int32(hgLifecycleDraining))
	state := &hgConnection{}
	conn := &hgMetricsTestConn{state: state}
	if server.hgActivateWebSocket("video-1", "user-1", conn, state) {
		t.Fatal("late upgrade was activated during drain")
	}
	if got := server.metrics.lateHandshakeRejections.Load(); got != 1 {
		t.Fatalf("late handshake rejections = %d, want 1", got)
	}
	if state.upgraded || state.videoID != "" || server.metrics.websocketConnections.Load() != 0 || server.metrics.activeRooms.Load() != 0 {
		t.Fatal("rejected late upgrade changed connection or room state")
	}
}

func TestOnOpenRejectsNewConnectionWhileDraining(t *testing.T) {
	server := &Server{config: ConfigPackage.HGVideoDanmakuConfig{MaxConnections: 100}}
	server.lifecycle.Store(int32(hgLifecycleDraining))
	conn := &hgMetricsTestConn{}
	_, action := server.OnOpen(conn)
	if action != gnet.Close {
		t.Fatalf("OnOpen() action = %v, want close", action)
	}
	if conn.state == nil || !conn.state.counted.Load() || server.connections.Load() != 1 {
		t.Fatal("rejected raw connection was not registered for one-time OnClose release")
	}
	server.OnClose(conn, nil)
	if got := server.connections.Load(); got != 0 {
		t.Fatalf("connections after OnClose = %d, want 0", got)
	}
}

func TestWaitForDrainReturnsWhenConnectionsReachZeroAndCountsTimeout(t *testing.T) {
	server := &Server{}
	server.metrics.websocketConnections.Store(1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		server.metrics.websocketConnections.Store(0)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.WaitForDrain(ctx); err != nil {
		t.Fatalf("WaitForDrain() error = %v", err)
	}

	server.metrics.websocketConnections.Store(1)
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer timeoutCancel()
	if err := server.WaitForDrain(timeoutCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForDrain() timeout error = %v", err)
	}
	if got := server.metrics.drainTimeouts.Load(); got != 1 {
		t.Fatalf("drain timeouts = %d, want 1", got)
	}
}

func TestActiveRoomGaugeTracksFirstJoinAndLastLeave(t *testing.T) {
	server := NewServer(nil, nil, ConfigPackage.HGVideoDanmakuConfig{RoomShardCount: 16, MemberShardCount: 4, BroadcastWorkerCount: 1, BroadcastQueueSize: 1})
	firstState, secondState := &hgConnection{}, &hgConnection{}
	first := &hgMetricsTestConn{state: firstState}
	second := &hgMetricsTestConn{state: secondState}

	server.hgJoin("video-1", first, firstState)
	server.hgJoin("video-1", second, secondState)
	if got := server.metrics.activeRooms.Load(); got != 1 {
		t.Fatalf("active rooms after two joins = %d, want 1", got)
	}
	server.hgLeave("video-1", first)
	if got := server.metrics.activeRooms.Load(); got != 1 {
		t.Fatalf("active rooms after first leave = %d, want 1", got)
	}
	server.hgLeave("video-1", second)
	if got := server.metrics.activeRooms.Load(); got != 0 {
		t.Fatalf("active rooms after last leave = %d, want 0", got)
	}
}

func TestOutboundPendingMetricsReleaseOnCompletionAndFailure(t *testing.T) {
	server := &Server{config: ConfigPackage.HGVideoDanmakuConfig{MaxPendingBytes: 1024}}
	state := &hgConnection{}
	callbackFailure := &hgMetricsTestConn{state: state, callbackErr: errors.New("write failed")}
	if err := server.hgWriteFrame(callbackFailure, state, []byte("frame")); err != nil {
		t.Fatalf("callback write returned error: %v", err)
	}
	if got := server.metrics.outboundPendingBytes.Load(); got != 0 {
		t.Fatalf("pending bytes after callback = %d, want 0", got)
	}
	if got := server.metrics.outboundFailures[hgOutboundFailureCallback].Load(); got != 1 {
		t.Fatalf("callback failures = %d, want 1", got)
	}

	immediateFailure := &hgMetricsTestConn{state: state, asyncWriteErr: errors.New("queue unavailable")}
	if err := server.hgWriteFrame(immediateFailure, state, []byte("frame")); err == nil {
		t.Fatal("immediate AsyncWrite failure was ignored")
	}
	if got := server.metrics.outboundPendingBytes.Load(); got != 0 {
		t.Fatalf("pending bytes after immediate failure = %d, want 0", got)
	}
	if got := server.metrics.outboundFailures[hgOutboundFailureAsyncWrite].Load(); got != 1 {
		t.Fatalf("AsyncWrite failures = %d, want 1", got)
	}

	state.pendingBytes.Store(1024)
	if err := server.hgWriteFrame(immediateFailure, state, []byte("x")); !errors.Is(err, hgErrPendingWriteLimit) {
		t.Fatalf("pending budget error = %v, want %v", err, hgErrPendingWriteLimit)
	}
	if got := server.metrics.outboundFailures[hgOutboundFailurePendingBudget].Load(); got != 1 {
		t.Fatalf("pending budget failures = %d, want 1", got)
	}
}

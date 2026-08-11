package VideoDanmakuRealtimePackage

import (
	VideoDanmakuDtoPackage "MLC_GO/internal/modules/video_danmaku/dto"
	ConfigPackage "MLC_GO/internal/pkg/config"
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panjf2000/gnet/v2"
)

type hgMetricsTestConn struct {
	gnet.Conn
	state         *hgConnection
	asyncWriteErr error
	callbackErr   error
}

func (c *hgMetricsTestConn) Context() any { return c.state }

func (c *hgMetricsTestConn) AsyncWrite(_ []byte, callback gnet.AsyncCallback) error {
	if c.asyncWriteErr != nil {
		return c.asyncWriteErr
	}
	if callback != nil {
		return callback(c, c.callbackErr)
	}
	return nil
}

func (c *hgMetricsTestConn) Close() error { return nil }

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

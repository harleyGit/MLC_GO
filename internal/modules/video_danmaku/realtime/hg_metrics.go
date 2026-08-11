package VideoDanmakuRealtimePackage

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

const (
	hgCommandRejectionInvalid = iota
	hgCommandRejectionRateLimited
	hgCommandRejectionQueueFull
	hgCommandRejectionCount
)

const (
	hgPublishSuccess = iota
	hgPublishFailure
	hgPublishResultCount
)

const (
	hgRecipientQueued = iota
	hgRecipientFailed
	hgRecipientResultCount
)

const (
	hgOutboundFailurePendingBudget = iota
	hgOutboundFailureAsyncWrite
	hgOutboundFailureCallback
	hgOutboundFailureReasonCount
)

var hgBroadcastDurationBounds = [...]time.Duration{
	time.Millisecond,
	5 * time.Millisecond,
	10 * time.Millisecond,
	25 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
}

// hgRealtimeMetrics 保存单个 realtime Server 的固定基数内存指标。
// 热路径只执行原子操作；Prometheus 抓取不会扫描连接、房间成员或访问外部依赖。
type hgRealtimeMetrics struct {
	websocketConnections atomic.Int64
	activeRooms          atomic.Int64
	outboundPendingBytes atomic.Int64

	commandsReceived      atomic.Uint64
	commandRejections     [hgCommandRejectionCount]atomic.Uint64
	commandCreateFailures atomic.Uint64
	publishResults        [hgPublishResultCount]atomic.Uint64
	broadcastQueueDropped atomic.Uint64
	broadcasts            atomic.Uint64
	recipientWrites       [hgRecipientResultCount]atomic.Uint64
	outboundFailures      [hgOutboundFailureReasonCount]atomic.Uint64

	broadcastDurationBuckets [len(hgBroadcastDurationBounds)]atomic.Uint64
	broadcastDurationCount   atomic.Uint64
	broadcastDurationNanos   atomic.Uint64
}

func (m *hgRealtimeMetrics) hgObserveBroadcastDuration(elapsed time.Duration) {
	if elapsed < 0 {
		elapsed = 0
	}
	m.broadcastDurationNanos.Add(uint64(elapsed))
	m.broadcastDurationCount.Add(1)
	for index, bound := range hgBroadcastDurationBounds {
		if elapsed <= bound {
			m.broadcastDurationBuckets[index].Add(1)
			return
		}
	}
}

// HGWritePrometheusMetrics 输出当前进程的弹幕实时指标。
// 所有标签均为固定枚举，禁止加入视频、用户、请求、连接或错误文本等高基数字段。
func (s *Server) HGWritePrometheusMetrics(w io.Writer) {
	if s == nil {
		return
	}
	hgWriteRealtimeGauge(w, "mlc_video_danmaku_websocket_connections", "Current authenticated and upgraded danmaku WebSocket connections in this process.", s.metrics.websocketConnections.Load())
	hgWriteRealtimeGauge(w, "mlc_video_danmaku_websocket_connection_limit", "Configured raw TCP connection admission limit for this danmaku process.", int64(s.config.MaxConnections))
	hgWriteRealtimeGauge(w, "mlc_video_danmaku_active_rooms", "Current local rooms with at least one upgraded WebSocket connection.", s.metrics.activeRooms.Load())
	hgWriteRealtimeGauge(w, "mlc_video_danmaku_command_queue_size", "Approximate current danmaku command queue occupancy.", int64(len(s.queue)))
	hgWriteRealtimeGauge(w, "mlc_video_danmaku_command_queue_capacity", "Configured danmaku command queue capacity.", int64(cap(s.queue)))
	broadcastQueueSize, broadcastQueueCapacity := 0, 0
	for _, queue := range s.broadcastQueue {
		broadcastQueueSize += len(queue)
		broadcastQueueCapacity += cap(queue)
	}
	hgWriteRealtimeGauge(w, "mlc_video_danmaku_broadcast_queue_size", "Approximate aggregate local broadcast queue occupancy.", int64(broadcastQueueSize))
	hgWriteRealtimeGauge(w, "mlc_video_danmaku_broadcast_queue_capacity", "Aggregate local broadcast queue capacity.", int64(broadcastQueueCapacity))
	hgWriteRealtimeGauge(w, "mlc_video_danmaku_outbound_pending_bytes", "Aggregate bytes reserved by pending gnet asynchronous writes.", s.metrics.outboundPendingBytes.Load())

	hgWriteRealtimeCounter(w, "mlc_video_danmaku_commands_received_total", "Complete danmaku WebSocket text commands received for parsing.", s.metrics.commandsReceived.Load())
	hgWriteRealtimeCounterSeries(w, "mlc_video_danmaku_command_rejections_total", "Danmaku commands rejected before persistence.", "reason", []string{"invalid_command", "rate_limited", "queue_full"}, s.metrics.commandRejections[:])
	hgWriteRealtimeCounter(w, "mlc_video_danmaku_command_create_failures_total", "Danmaku command persistence failures.", s.metrics.commandCreateFailures.Load())
	hgWriteRealtimeCounterSeries(w, "mlc_video_danmaku_broadcast_publish_total", "Post-commit Redis danmaku publication results.", "result", []string{"success", "failure"}, s.metrics.publishResults[:])
	hgWriteRealtimeCounter(w, "mlc_video_danmaku_broadcast_queue_dropped_total", "Redis room events dropped because the bounded local broadcast queue was full.", s.metrics.broadcastQueueDropped.Load())
	hgWriteRealtimeCounter(w, "mlc_video_danmaku_broadcasts_total", "Local room broadcast events processed by this process.", s.metrics.broadcasts.Load())
	hgWriteRealtimeCounterSeries(w, "mlc_video_danmaku_broadcast_recipient_writes_total", "Local recipient writes accepted or rejected by gnet scheduling.", "result", []string{"queued", "failed"}, s.metrics.recipientWrites[:])
	hgWriteRealtimeCounterSeries(w, "mlc_video_danmaku_outbound_write_failures_total", "Danmaku outbound write failures by fixed reason.", "reason", []string{"pending_budget", "async_write", "callback"}, s.metrics.outboundFailures[:])
	s.hgWriteBroadcastDurationHistogram(w)
}

func (s *Server) hgWriteBroadcastDurationHistogram(w io.Writer) {
	const name = "mlc_video_danmaku_broadcast_duration_seconds"
	_, _ = fmt.Fprintf(w, "# HELP %s Time spent encoding and scheduling one local room fanout; excludes socket flush and client rendering.\n# TYPE %s histogram\n", name, name)
	var cumulative uint64
	for index, bound := range hgBroadcastDurationBounds {
		cumulative += s.metrics.broadcastDurationBuckets[index].Load()
		_, _ = fmt.Fprintf(w, "%s_bucket{le=\"%g\"} %d\n", name, bound.Seconds(), cumulative)
	}
	count := s.metrics.broadcastDurationCount.Load()
	_, _ = fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n", name, count)
	_, _ = fmt.Fprintf(w, "%s_sum %g\n", name, float64(s.metrics.broadcastDurationNanos.Load())/float64(time.Second))
	_, _ = fmt.Fprintf(w, "%s_count %d\n", name, count)
}

func hgWriteRealtimeGauge(w io.Writer, name, help string, value int64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, value)
}

func hgWriteRealtimeCounter(w io.Writer, name, help string, value uint64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
}

func hgWriteRealtimeCounterSeries(w io.Writer, name, help, label string, values []string, counters []atomic.Uint64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", name, help, name)
	for index, value := range values {
		_, _ = fmt.Fprintf(w, "%s{%s=%q} %d\n", name, label, value, counters[index].Load())
	}
}

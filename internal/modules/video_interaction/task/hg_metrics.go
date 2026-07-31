package VideoInteractionTaskPackage

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

type hgProjectionMetric struct {
	runs       atomic.Uint64
	rows       atomic.Uint64
	failures   atomic.Uint64
	leaseSkips atomic.Uint64
	duration   atomic.Uint64
}

// HGProjectionMetricSnapshot 是四条固定重投影流的只读低基数指标快照。
type HGProjectionMetricSnapshot struct {
	Stream        string
	Runs          uint64
	Rows          uint64
	Failures      uint64
	LeaseSkips    uint64
	DurationNanos uint64
}

var hgProjectionMetrics = map[HGProjectionStream]*hgProjectionMetric{
	HGProjectionStreamVideoState:   {},
	HGProjectionStreamFollowState:  {},
	HGProjectionStreamVideoCounts:  {},
	HGProjectionStreamFollowCounts: {},
}

func hgObserveProjection(stream HGProjectionStream, rows int, elapsed time.Duration, err error, acquired bool) {
	metric := hgProjectionMetrics[stream]
	if metric == nil {
		return
	}
	metric.runs.Add(1)
	metric.rows.Add(uint64(rows))
	metric.duration.Add(uint64(elapsed))
	if err != nil {
		metric.failures.Add(1)
	}
	if !acquired && err == nil {
		metric.leaseSkips.Add(1)
	}
}

// HGWritePrometheusMetrics 仅输出四个固定 stream 标签，禁止加入用户、视频、token 或错误文本。
func HGWritePrometheusMetrics(w io.Writer) {
	for _, stream := range hgProjectionStreams {
		metric := hgProjectionMetrics[stream]
		label := fmt.Sprintf(`{stream="%s"}`, stream)
		_, _ = fmt.Fprintf(w, "mlc_interaction_reproject_runs_total%s %d\n", label, metric.runs.Load())
		_, _ = fmt.Fprintf(w, "mlc_interaction_reproject_rows_total%s %d\n", label, metric.rows.Load())
		_, _ = fmt.Fprintf(w, "mlc_interaction_reproject_failures_total%s %d\n", label, metric.failures.Load())
		_, _ = fmt.Fprintf(w, "mlc_interaction_reproject_lease_skips_total%s %d\n", label, metric.leaseSkips.Load())
		_, _ = fmt.Fprintf(w, "mlc_interaction_reproject_duration_nanoseconds_total%s %d\n", label, metric.duration.Load())
	}
}

// HGProjectionMetricsSnapshot 复制原子计数，调用方无法修改 worker 内部状态。
func HGProjectionMetricsSnapshot() []HGProjectionMetricSnapshot {
	result := make([]HGProjectionMetricSnapshot, 0, len(hgProjectionStreams))
	for _, stream := range hgProjectionStreams {
		metric := hgProjectionMetrics[stream]
		result = append(result, HGProjectionMetricSnapshot{Stream: string(stream), Runs: metric.runs.Load(), Rows: metric.rows.Load(), Failures: metric.failures.Load(), LeaseSkips: metric.leaseSkips.Load(), DurationNanos: metric.duration.Load()})
	}
	return result
}

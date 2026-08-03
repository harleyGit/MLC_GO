package VideoCommentTaskPackage

import (
	"fmt"
	"io"
	"sync/atomic"
)

var (
	// 两个 gauge 保存最近一次有界数据库观测结果；三个 counter 只累计低基数进程事件，不包含用户、评论或错误文本标签。
	hgReactionDirtyOldestAgeSeconds atomic.Uint64
	hgImageCleanupOldestAgeSeconds  atomic.Uint64
	hgReactionProjectionCASMisses   atomic.Uint64
	hgExpiredLeaseReclaims          atomic.Uint64
	hgImageCleanupFailures          atomic.Uint64
)

// HGWritePrometheusMetrics 输出评论维护的低基数内存快照，不在 Prometheus 抓取路径访问数据库或对象存储。
// 多副本部署应按 pod 抓取；年龄 gauge 可取 max，counter 使用 rate 后聚合。
func HGWritePrometheusMetrics(w io.Writer) {
	hgWriteGauge(w, "mlc_video_comment_reaction_dirty_oldest_age_seconds", "Age in seconds of the oldest pending reaction projection.", hgReactionDirtyOldestAgeSeconds.Load())
	hgWriteGauge(w, "mlc_video_comment_image_cleanup_oldest_age_seconds", "Age in seconds of the oldest eligible image cleanup item.", hgImageCleanupOldestAgeSeconds.Load())
	hgWriteCounter(w, "mlc_video_comment_reaction_projection_cas_misses_total", "Reaction projection revision CAS misses.", hgReactionProjectionCASMisses.Load())
	hgWriteCounter(w, "mlc_video_comment_image_cleanup_expired_lease_reclaims_total", "Image cleanup claims recovered from expired leases.", hgExpiredLeaseReclaims.Load())
	hgWriteCounter(w, "mlc_video_comment_image_cleanup_failures_total", "Image cleanup storage or completion failures.", hgImageCleanupFailures.Load())
}

func hgWriteGauge(w io.Writer, name, help string, value uint64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, value)
}

func hgWriteCounter(w io.Writer, name, help string, value uint64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
}

func hgResetVideoCommentMaintenanceMetricsForTest() {
	hgReactionDirtyOldestAgeSeconds.Store(0)
	hgImageCleanupOldestAgeSeconds.Store(0)
	hgReactionProjectionCASMisses.Store(0)
	hgExpiredLeaseReclaims.Store(0)
	hgImageCleanupFailures.Store(0)
}

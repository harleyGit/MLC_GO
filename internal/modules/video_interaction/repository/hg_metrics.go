/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-31 13:08:51
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-10-16 16:36:40
 * @FilePath: /MLC_GO/internal/modules/video_interaction/repository/hg_metrics.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package VideoInteractionRepositoryPackage

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

var (// 调用 .Add 通常是原子计数操作（具体取决于变量定义），支持并发安全地累加。
	hgPersistenceBatches  atomic.Uint64 // 累计处理批次数
	hgPersistenceEvents   atomic.Uint64 // 累计尝试处理事件数量, 该记录的是传入的事件数，不代表实际成功写入数据库的记录数，因为可能存在重复事件或者事务失败。
	hgPersistenceFailures atomic.Uint64// 累计处理失败批次数
	hgPersistenceNanos    atomic.Uint64 // 累计数据库处理耗时（纳秒）
	hgInboxDuplicates     atomic.Uint64
	hgInboxConflicts      atomic.Uint64
	hgWalletDebits        atomic.Uint64
)

// hgObservePersistenceBatch 记录监控指标
//
//	@param events 
//	@param elapsed 计算处理耗时
//	@param err 
func hgObservePersistenceBatch(events int, elapsed time.Duration, err error) {
	hgPersistenceBatches.Add(1)
	hgPersistenceEvents.Add(uint64(events))
	hgPersistenceNanos.Add(uint64(elapsed))
	if err != nil {
		hgPersistenceFailures.Add(1)
	}
}

func hgObserveInboxDuplicate() { hgInboxDuplicates.Add(1) }
func hgObserveInboxConflict()  { hgInboxConflicts.Add(1) }
func hgObserveWalletDebit()    { hgWalletDebits.Add(1) }

// HGWritePrometheusMetrics 输出 Interaction 持久化低基数进程指标。
func HGWritePrometheusMetrics(w io.Writer) {
	metrics := []struct {
		name  string
		help  string
		value uint64
	}{
		{"mlc_interaction_persistence_batches_total", "Interaction persistence transactions.", hgPersistenceBatches.Load()},
		{"mlc_interaction_persistence_events_total", "Interaction events included in persistence transactions.", hgPersistenceEvents.Load()},
		{"mlc_interaction_persistence_failures_total", "Failed interaction persistence transactions.", hgPersistenceFailures.Load()},
		{"mlc_interaction_persistence_batch_duration_nanoseconds_total", "Total interaction persistence duration in nanoseconds.", hgPersistenceNanos.Load()},
		{"mlc_interaction_inbox_duplicates_total", "Exact interaction event replays.", hgInboxDuplicates.Load()},
		{"mlc_interaction_inbox_conflicts_total", "Conflicting interaction inbox uniqueness collisions.", hgInboxConflicts.Load()},
		{"mlc_interaction_wallet_debits_total", "Committed coin wallet debits.", hgWalletDebits.Load()},
	}
	for _, metric := range metrics {
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", metric.name, metric.help, metric.name, metric.name, metric.value)
	}
}

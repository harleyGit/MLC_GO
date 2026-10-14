/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-27 17:10:45
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-10-14 20:30:58
 * @FilePath: /MLC_GO/internal/consumer/statistic/hg_metrics.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 * Copyright (c) 2026 by GangHuang harleysor@qq.com, All Rights Reserved.

 Metrics（指标）就是用来监控程序运行状态的数据。
例如：接口调用了多少次、数据库写入失败了多少次、Redis 出现了多少次错误等。

流程大致是：
+-----------------------------------+
|           Go 业务服务             |
|                                   |
|      Redis / ClickHouse 操作      |
+-----------------------------------+
                 |
                 v
+-----------------------------------+
|         Metrics 指标采集          |
|                                   |
|       Add(1) / Load()             |
+-----------------------------------+
                 |
                 v
+-----------------------------------+
|         HTTP /metrics 接口       |
|                                   |
|       输出 Prometheus 格式       |
+-----------------------------------+
                 |
                 | 定期拉取
                 v
+-----------------------------------+
|            Prometheus             |
|                                   |
|        采集、存储、查询指标       |
+-----------------------------------+
                 |
                 v
+-----------------------------------+
|             Grafana               |
|                                   |
|        可视化图表 / 监控面板      |
+-----------------------------------+

对于你的 Go 后端工程，可以这样理解：
- Logs（日志）：记录某一次操作发生了什么，例如某条 SQL 执行失败。
- Metrics（指标）：统计整个系统运行得怎么样，例如过去 5 分钟 Redis 失败了多少次。
- Prometheus：定期采集并保存 Metrics 数据。
- Grafana：将 Metrics 展示成图表，方便观察。
一句话：Metrics 相当于 Go 服务的运行仪表盘数据源，让你不用一直翻日志，也能了解系统运行是否正常。
 */
package statistic


import (
	"fmt"
	"io"
	"sync/atomic"
)

var (
	hgStatisticAuthorityWrites       atomic.Uint64
	hgStatisticAuthorityWriteFailure atomic.Uint64
	hgStatisticRedisFailures         atomic.Uint64
	hgStatisticReconcileRuns         atomic.Uint64
	hgStatisticReconcileFailures     atomic.Uint64
	hgStatisticReconcileMismatches   atomic.Uint64
	hgStatisticReconcileLastSuccess  atomic.Int64
	hgStatisticReconcileCurrentDrift atomic.Uint64
)

// HGStatisticMetrics 是 Statistic 权威写入和对账的进程级指标快照。
type HGStatisticMetrics struct {
	AuthorityWrites       uint64 // ClickHouse 成功写入次数
	AuthorityWriteFailure uint64
	RedisFailures         uint64 // Redis 操作失败次数
	ReconcileRuns         uint64 // 数据对账执行次数
	ReconcileFailures     uint64 // 对账失败次数
	ReconcileMismatches   uint64 // 对账发现不一致的次数
	ReconcileLastSuccess  int64 // 最近一次对账成功的 Unix 时间戳
	CurrentDrift          uint64 // Redis 和 ClickHouse 当前数据偏差
}

// HGStatisticMetricsSnapshot 主要作用是：收集业务运行指标，并转换成 Prometheus 能识别的格式，方便监控和告警。
func HGStatisticMetricsSnapshot() HGStatisticMetrics {
	return HGStatisticMetrics{
		// Load() 通常是 Go 原子变量（atomic.Uint64 等）提供的方法。
		AuthorityWrites: hgStatisticAuthorityWrites.Load(), AuthorityWriteFailure: hgStatisticAuthorityWriteFailure.Load(),
		RedisFailures: hgStatisticRedisFailures.Load(), ReconcileRuns: hgStatisticReconcileRuns.Load(),
		ReconcileFailures: hgStatisticReconcileFailures.Load(), ReconcileMismatches: hgStatisticReconcileMismatches.Load(),
		ReconcileLastSuccess: hgStatisticReconcileLastSuccess.Load(), CurrentDrift: hgStatisticReconcileCurrentDrift.Load(),
	}
}

// HGWritePrometheusMetrics 输出固定低基数的 Statistic 指标。
func HGWritePrometheusMetrics(w io.Writer) {
	metrics := HGStatisticMetricsSnapshot()
	// writeCounter() 函数用于输出 Prometheus 格式的计数器指标。是一个内部函数，用于输出 Prometheus 格式的计数器指标。
	// Counter 记录累计发生了多少次
	//  # HELP：指标说明。
	// # TYPE：指标类型。
	// 最后一行：指标名称和实际数值。
	writeCounter := func(name string, help string, value uint64) {
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
	}
	// 仪表值，Gauge 用来记录当前状态，可以增加，也可以减少
	// Gauge 记录现在是多少
	writeGauge := func(name string, help string, value int64) {
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, value)
	}
	writeCounter("mlc_statistic_clickhouse_writes_total", "Statistic authority events acknowledged by ClickHouse.", metrics.AuthorityWrites)
	writeCounter("mlc_statistic_clickhouse_write_failures_total", "Statistic authority writes rejected or unavailable.", metrics.AuthorityWriteFailure)
	writeCounter("mlc_statistic_redis_projection_failures_total", "Statistic Redis projection failures after authority writes.", metrics.RedisFailures)
	writeCounter("mlc_statistic_reconcile_runs_total", "Statistic Redis and ClickHouse reconciliation runs.", metrics.ReconcileRuns)
	writeCounter("mlc_statistic_reconcile_failures_total", "Statistic reconciliation failures.", metrics.ReconcileFailures)
	writeCounter("mlc_statistic_reconcile_mismatches_total", "Statistic dimensions observed with drift.", metrics.ReconcileMismatches)
	writeGauge("mlc_statistic_reconcile_last_success_unixtime", "Unix time of the last successful statistic reconciliation.", metrics.ReconcileLastSuccess)
	writeGauge("mlc_statistic_reconcile_current_drift", "Current absolute Redis versus ClickHouse statistic drift.", int64(metrics.CurrentDrift))
}

func hgResetStatisticMetricsForTest() {
	hgStatisticAuthorityWrites.Store(0)
	hgStatisticAuthorityWriteFailure.Store(0)
	hgStatisticRedisFailures.Store(0)
	hgStatisticReconcileRuns.Store(0)
	hgStatisticReconcileFailures.Store(0)
	hgStatisticReconcileMismatches.Store(0)
	hgStatisticReconcileLastSuccess.Store(0)
	hgStatisticReconcileCurrentDrift.Store(0)
}

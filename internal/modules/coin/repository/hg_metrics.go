package CoinRepositoryPackage

import (
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type hgCoinMetric struct {
	success atomic.Uint64
	failure atomic.Uint64
	nanos   atomic.Uint64
}

var (
	hgCoinMetricsMu sync.Mutex
	hgCoinMetrics   = map[CoinModelPackage.HGOperation]*hgCoinMetric{}
)

func hgObserveCoinMutation(operation CoinModelPackage.HGOperation, committed bool, elapsed time.Duration, err error) {
	hgCoinMetricsMu.Lock()
	metric := hgCoinMetrics[operation]
	if metric == nil {
		metric = &hgCoinMetric{}
		hgCoinMetrics[operation] = metric
	}
	hgCoinMetricsMu.Unlock()
	metric.nanos.Add(uint64(elapsed))
	if err == nil && committed {
		metric.success.Add(1)
	} else if err != nil {
		metric.failure.Add(1)
	}
}

// HGWritePrometheusMetrics 仅输出固定 operation/result 标签。
// 禁止加入 user_id、request_id、business_key 或错误文本，避免资产热点路径产生高基数时间序列。
func HGWritePrometheusMetrics(w io.Writer) {
	operations := [...]CoinModelPackage.HGOperation{CoinModelPackage.HGOperationInitialize, CoinModelPackage.HGOperationRecharge, CoinModelPackage.HGOperationGrant, CoinModelPackage.HGOperationDebit, CoinModelPackage.HGOperationRefund, CoinModelPackage.HGOperationExpire, CoinModelPackage.HGOperationCorrection}
	for _, operation := range operations {
		hgCoinMetricsMu.Lock()
		metric := hgCoinMetrics[operation]
		hgCoinMetricsMu.Unlock()
		if metric == nil {
			metric = &hgCoinMetric{}
		}
		_, _ = fmt.Fprintf(w, "mlc_coin_mutations_total{operation=\"%s\",result=\"success\"} %d\n", operation, metric.success.Load())
		_, _ = fmt.Fprintf(w, "mlc_coin_mutations_total{operation=\"%s\",result=\"failure\"} %d\n", operation, metric.failure.Load())
		_, _ = fmt.Fprintf(w, "mlc_coin_mutation_duration_nanoseconds_total{operation=\"%s\"} %d\n", operation, metric.nanos.Load())
	}
}

package statistic

import (
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	"context"
	"testing"
	"time"
)

type hgTotalsReaderStub struct {
	totals map[ClickHousePackage.HGStatisticDimension]uint64
	err    error
}

func (s hgTotalsReaderStub) GetStatisticTotals(context.Context, string) (map[ClickHousePackage.HGStatisticDimension]uint64, error) {
	return s.totals, s.err
}

type hgHashReaderStub struct {
	values map[string]map[string]string
}

func (s hgHashReaderStub) HGetAll(_ context.Context, key string) (map[string]string, error) {
	return s.values[key], nil
}

func TestHGReconcilerDetectsRedisClickHouseDriftWithoutRepairing(t *testing.T) {
	hgResetStatisticMetricsForTest()
	reconciler := NewHGReconciler(
		hgTotalsReaderStub{totals: map[ClickHousePackage.HGStatisticDimension]uint64{
			{Shard: 0, EventName: "video.published"}: 12,
		}},
		hgHashReaderStub{values: map[string]map[string]string{
			"statistic:v2:{stat-0000}:events": {"video.published": "10"},
		}},
		HGReconcileConfig{Generation: "v2", ShardCount: 1, Timeout: time.Second},
	)
	result, err := reconciler.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.MismatchedDimensions != 1 || result.AbsoluteDrift != 2 {
		t.Fatalf("result = %#v, want one mismatch and drift 2", result)
	}
	snapshot := HGStatisticMetricsSnapshot()
	if snapshot.ReconcileRuns != 1 || snapshot.ReconcileMismatches != 1 || snapshot.CurrentDrift != 2 {
		t.Fatalf("metrics = %#v", snapshot)
	}
}

func TestHGReconcilerTreatsMissingRedisFieldsAsZero(t *testing.T) {
	reconciler := NewHGReconciler(
		hgTotalsReaderStub{totals: map[ClickHousePackage.HGStatisticDimension]uint64{{Shard: 0, EventName: "video.deleted"}: 3}},
		hgHashReaderStub{},
		HGReconcileConfig{Generation: "v2", ShardCount: 1, Timeout: time.Second},
	)
	result, err := reconciler.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.AbsoluteDrift != 3 {
		t.Fatalf("result = %#v, want drift 3", result)
	}
}

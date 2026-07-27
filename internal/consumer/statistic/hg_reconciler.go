package statistic

import (
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"fmt"
	"strconv"
	"time"
)

// AggregateReader 读取 ClickHouse 精确去重后的权威累计值。
type AggregateReader interface {
	GetStatisticTotals(ctx context.Context, generation string) (map[ClickHousePackage.HGStatisticDimension]uint64, error)
}

// RedisHashReader 是对账所需的最小 Redis Hash 读取接口。
type RedisHashReader interface {
	HGetAll(ctx context.Context, key string) (map[string]string, error)
}

// HGReconcileConfig 描述一次检测式对账的边界。
type HGReconcileConfig struct {
	Generation string
	ShardCount int
	Timeout    time.Duration
}

// HGReconcileResult 描述本轮漂移，不执行在线修复。
type HGReconcileResult struct {
	MismatchedDimensions uint64
	AbsoluteDrift        uint64
}

// HGReconciler 检测 Redis 投影与 ClickHouse 权威累计值的差异。
type HGReconciler struct {
	authority AggregateReader
	redis     RedisHashReader
	config    HGReconcileConfig
}

// NewHGReconciler 创建检测式对账器。
func NewHGReconciler(authority AggregateReader, redis RedisHashReader, config HGReconcileConfig) *HGReconciler {
	return &HGReconciler{authority: authority, redis: redis, config: config}
}

// Reconcile 比较所有 shard 和事件维度；缺失值按零处理，且不覆盖 Redis。
func (r *HGReconciler) Reconcile(ctx context.Context) (HGReconcileResult, error) {
	hgStatisticReconcileRuns.Add(1)
	if r == nil || r.authority == nil || r.redis == nil || r.config.ShardCount <= 0 || r.config.Timeout <= 0 {
		hgStatisticReconcileFailures.Add(1)
		return HGReconcileResult{}, fmt.Errorf("statistic reconciler dependencies are invalid")
	}
	reconcileCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()
	authorityTotals, err := r.authority.GetStatisticTotals(reconcileCtx, r.config.Generation)
	if err != nil {
		hgStatisticReconcileFailures.Add(1)
		return HGReconcileResult{}, fmt.Errorf("read clickhouse statistic totals: %w", err)
	}
	redisTotals := make(map[ClickHousePackage.HGStatisticDimension]uint64)
	for shard := 0; shard < r.config.ShardCount; shard++ {
		values, err := r.redis.HGetAll(reconcileCtx, PersistenceRedisPackage.GetVideoEventCounterKey(r.config.Generation, shard))
		if err != nil {
			hgStatisticReconcileFailures.Add(1)
			return HGReconcileResult{}, fmt.Errorf("read redis statistic shard %d: %w", shard, err)
		}
		for eventName, rawValue := range values {
			value, err := strconv.ParseUint(rawValue, 10, 64)
			if err != nil {
				hgStatisticReconcileFailures.Add(1)
				return HGReconcileResult{}, fmt.Errorf("parse redis statistic shard %d event %s: %w", shard, eventName, err)
			}
			redisTotals[ClickHousePackage.HGStatisticDimension{Shard: shard, EventName: eventName}] = value
		}
	}
	dimensions := make(map[ClickHousePackage.HGStatisticDimension]struct{}, len(authorityTotals)+len(redisTotals))
	for dimension := range authorityTotals {
		dimensions[dimension] = struct{}{}
	}
	for dimension := range redisTotals {
		dimensions[dimension] = struct{}{}
	}
	var result HGReconcileResult
	for dimension := range dimensions {
		authorityValue := authorityTotals[dimension]
		redisValue := redisTotals[dimension]
		if authorityValue == redisValue {
			continue
		}
		result.MismatchedDimensions++
		if authorityValue > redisValue {
			result.AbsoluteDrift += authorityValue - redisValue
		} else {
			result.AbsoluteDrift += redisValue - authorityValue
		}
	}
	hgStatisticReconcileMismatches.Add(result.MismatchedDimensions)
	hgStatisticReconcileCurrentDrift.Store(result.AbsoluteDrift)
	hgStatisticReconcileLastSuccess.Store(time.Now().UTC().Unix())
	return result, nil
}

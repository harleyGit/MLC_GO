/*
Reconcile() 就像财务系统中的对账员：
- ClickHouse 是权威账本。
- Redis 是另一份业务统计账本。
- HGStatisticDimension 是对账项目（分片 + 事件名称）。
- MismatchedDimensions 是有多少项账目不一致。
- AbsoluteDrift 是所有账目差异的绝对值总和。
*/

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

// Reconcile() 是一个数据对账函数，主要用于比较 ClickHouse 和 Redis 中的业务事件统计数据是否一致
// 例如：ClickHouse 记录某视频点击了 100 次，而 Redis 记录了 95 次，那么两者相差 5 次，需要被发现。
/* 本方法执行流程：
         Reconcile() 开始
                 |
                 v
         对账次数 +1
                 |
                 v
          检查依赖配置
                 |
                 v
          设置超时时间
                 |
                 v
     查询 ClickHouse 统计数据
                 |
                 v
     遍历所有 Redis Shard
                 |
                 v
     HGetAll 读取事件计数
                 |
                 v
     合并两边所有统计维度
                 |
                 v
     逐个比较 ClickHouse / Redis
                 |
      +----------+----------+
      |                     |
      v                     v
   数据相同               数据不同
      |                     |
      v                     v
   跳过比较            不一致维度 +1
                            |
                            v
                       计算差值
      |                     |
      +----------+----------+
                 |
                 v
          更新 Metrics 指标
                 |
                 v
         返回对账结果
*/
func (r *HGReconciler) Reconcile(ctx context.Context) (HGReconcileResult, error) {

	// 记录对账次数；每执行一次 Reconcile()，对账次数就增加 1；即使后面执行失败，也会记录本次尝试。
	hgStatisticReconcileRuns.Add(1)

	// 检查依赖是否有效
	// r == nil：对账对象不存在。
	// authority == nil：ClickHouse 数据访问依赖不存在。
	// redis == nil：Redis 数据访问依赖不存在。
	// ShardCount <= 0：Redis 分片数量无效。
	// Timeout <= 0：超时时间无效。
	if r == nil || r.authority == nil || r.redis == nil || r.config.ShardCount <= 0 || r.config.Timeout <= 0 {

		// hgStatisticReconcileFailures 表示失败了多少次
		hgStatisticReconcileFailures.Add(1)
		return HGReconcileResult{}, fmt.Errorf("statistic reconciler dependencies are invalid")
	}

	// WithTimeout 给本次 Reconcile 操作设置一个最大执行时间，超过 r.config.Timeout 后，Go 会自动取消 reconcileCtx。
	reconcileCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()
	// 查询 ClickHouse 统计数据，这里 authority 表示权威数据源，在你的代码中是 ClickHouse
	authorityTotals, err := r.authority.GetStatisticTotals(reconcileCtx, r.config.Generation)
	if err != nil {
		hgStatisticReconcileFailures.Add(1)
		return HGReconcileResult{}, fmt.Errorf("read clickhouse statistic totals: %w", err)
	}
	// 查询所有 Redis 分片； 创建一个 Map，用来保存 Redis 统计数据
	redisTotals := make(map[ClickHousePackage.HGStatisticDimension]uint64)
	for shard := 0; shard < r.config.ShardCount; shard++ {
		// HGetAll 是 Redis Hash 命令，作用是读取指定 Hash Key 中的全部字段和值。
		values, err := r.redis.HGetAll(reconcileCtx, PersistenceRedisPackage.GetVideoEventCounterKey(r.config.Generation, shard))
		if err != nil {
			hgStatisticReconcileFailures.Add(1)
			return HGReconcileResult{}, fmt.Errorf("read redis statistic shard %d: %w", shard, err)
		}
		for eventName, rawValue := range values {
			// 将 Redis 返回的字符串（例如 "95"）转换为 uint64 数字（95）。
			value, err := strconv.ParseUint(rawValue, 10, 64)
			if err != nil {
				hgStatisticReconcileFailures.Add(1)
				return HGReconcileResult{}, fmt.Errorf("parse redis statistic shard %d event %s: %w", shard, eventName, err)
			}
			redisTotals[ClickHousePackage.HGStatisticDimension{Shard: shard, EventName: eventName}] = value
		}
	}
	// 合并所有统计维度
	// 为什么需要合并？
	// 因为 ClickHouse 和 Redis 中的事件不一定完全相同。
	dimensions := make(map[ClickHousePackage.HGStatisticDimension]struct{}, len(authorityTotals)+len(redisTotals))
	for dimension := range authorityTotals {
		dimensions[dimension] = struct{}{}
	}
	for dimension := range redisTotals {
		dimensions[dimension] = struct{}{}
	}
	var result HGReconcileResult
	// 比较两边的数据
	for dimension := range dimensions {
		authorityValue := authorityTotals[dimension]
		redisValue := redisTotals[dimension]
		if authorityValue == redisValue {
			continue
		}
		result.MismatchedDimensions++
		// 注意：AbsoluteDrift 不是两个系统总数相减，而是每个维度先取绝对差值，再求和，避免正负差异互相抵消。
		// 另外，Go 读取 Map 中不存在的 uint64 值时会返回 0，所以这段代码会把缺失维度按零计数处理
		if authorityValue > redisValue {
			result.AbsoluteDrift += authorityValue - redisValue
		} else {
			result.AbsoluteDrift += redisValue - authorityValue
		}
	}
	// 更新 Metrics 监控指标
	// 这里 Add() 是累计增加，而 Store() 是覆盖旧值。
	hgStatisticReconcileMismatches.Add(result.MismatchedDimensions) // 累计发现的不一致维度数量
	hgStatisticReconcileCurrentDrift.Store(result.AbsoluteDrift)    // 本次对账的绝对差值总和
	hgStatisticReconcileLastSuccess.Store(time.Now().UTC().Unix())  // 最近一次成功完成对账的 Unix 时间戳
	return result, nil
}

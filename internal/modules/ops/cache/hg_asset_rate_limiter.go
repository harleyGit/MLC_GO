package OpsCachePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
	"math"
	"time"
)

const (
	hgAssetWriteCapacity = int64(10)
	hgAssetWriteWindow   = time.Minute
)

// Allow enforces both operator and source-IP token buckets. Any Redis error is returned so callers fail closed.
func (c *Cache) Allow(ctx context.Context, operatorID, sourceIP string) (bool, error) {
	if c == nil || c.redisService == nil || c.redisService.Client() == nil {
		return false, errors.New("redis unavailable")
	}
	keys := []string{
		PersistenceRedisPackage.OpsAssetWriteOperatorRateKeyPrefix + operatorID,
		PersistenceRedisPackage.OpsAssetWriteIPRateKeyPrefix + sourceIP,
	}
	refillRate := float64(hgAssetWriteCapacity) / hgAssetWriteWindow.Seconds()
	ttlSeconds := int(math.Ceil(hgAssetWriteWindow.Seconds() * 2))
	for _, key := range keys {
		allowed, err := c.redisService.Client().Eval(ctx, PersistenceRedisPackage.TokenBucketRateLimitLuaScript, []string{key}, hgAssetWriteCapacity, refillRate, time.Now().UnixMilli(), 1, ttlSeconds).Int64()
		if err != nil {
			return false, err
		}
		if allowed != 1 {
			return false, nil
		}
	}
	return true, nil
}

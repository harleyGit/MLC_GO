/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 20:11:48
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-24 23:24:14
 * @FilePath: /MLC_GO/internal/modules/user/cache/hg_code_cache.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserCachePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	"context"
	"time"
)

type HGCodeCache struct {
	redisService *PersistenceRedisPackage.RedisService
}

func NewCodeCache(redisService *PersistenceRedisPackage.RedisService) *HGCodeCache {
	return &HGCodeCache{redisService: redisService}
}

func (c *HGCodeCache) SaveMultiportConcrolCache(ctx context.Context, uid int64,
	device string, jti string, ttl time.Duration) error {
	loginCodeKey := PersistenceRedisPackage.GetMultiportKey(uid, device)
	return c.redisService.SetToRedisV2(
		loginCodeKey,
		jti,
		int64(ttl),
		ctx,
	)
}

func (c *HGCodeCache) GetMultiportConcrolCache(ctx context.Context, uid int64,
	device string) (string, error) {
	loginCodeKey := PersistenceRedisPackage.GetMultiportKey(uid, device)
	return c.redisService.GetFromRedisV2(
		loginCodeKey,
		ctx,
	)
}

func (c *HGCodeCache) SetCode(ctx context.Context, phone, code string) error {
	loginCodeKey := PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.LoginCodeKey, phone)
	return c.redisService.SetToRedisV2(
		loginCodeKey,
		code,
		300,
		ctx,
	)
}

func (c *HGCodeCache) GetCode(ctx context.Context, phone string) (string, error) {

	loginCodeKey := PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.LoginCodeKey, phone)
	return c.redisService.GetFromRedisV2(loginCodeKey, ctx)
}

func (c *HGCodeCache) DeleteCode(ctx context.Context, phone string) {

	loginCodeKey := PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.LoginCodeKey, phone)
	c.redisService.DeleteFromRedis(loginCodeKey, ctx)
}

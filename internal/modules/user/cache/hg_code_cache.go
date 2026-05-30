/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 20:11:48
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-26 21:47:01
 * @FilePath: /MLC_GO/internal/modules/user/cache/hg_code_cache.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package cache

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrCodeCacheRedisNil 表示验证码或 token 缓存缺少 Redis 依赖。
var ErrCodeCacheRedisNil = errors.New("code cache redis dependency is nil")

type HGCodeCache struct {
	redisService *PersistenceRedisPackage.RedisService
}

// NewCodeCache 创建验证码和多端登录控制缓存访问对象。
func NewCodeCache(redisService *PersistenceRedisPackage.RedisService) *HGCodeCache {
	return &HGCodeCache{redisService: redisService}
}

// SaveMultiportConcrolCache 保存指定用户和设备的当前 token jti。
func (c *HGCodeCache) SaveMultiportConcrolCache(ctx context.Context, uid int64,
	device string, jti string, ttl time.Duration) error {
	if c == nil || c.redisService == nil {
		return ErrCodeCacheRedisNil
	}

	loginCodeKey := PersistenceRedisPackage.GetMultiportKey(uid, device)
	return c.redisService.SetToRedisV2(
		loginCodeKey,
		jti,
		ttl,
		ctx,
	)
}

// GetMultiportConcrolCache 获取指定用户和设备的当前 token jti。
func (c *HGCodeCache) GetMultiportConcrolCache(ctx context.Context, uid int64,
	device string) (string, error) {
	if c == nil || c.redisService == nil {
		return "", ErrCodeCacheRedisNil
	}

	loginCodeKey := PersistenceRedisPackage.GetMultiportKey(uid, device)
	val, err := c.redisService.GetFromRedisV2(
		loginCodeKey,
		ctx,
	)
	return decodeRedisCacheStringValue(val), err
}

// SetCode 缓存短信验证码，默认 5 分钟过期。
func (c *HGCodeCache) SetCode(ctx context.Context, phone, code string) error {
	if c == nil || c.redisService == nil {
		return ErrCodeCacheRedisNil
	}

	loginCodeKey := PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.LoginCodeKey, phone)
	return c.redisService.SetToRedisV2(
		loginCodeKey,
		code,
		5*time.Minute,
		ctx,
	)
}

// GetCode 读取短信验证码，并兼容 Redis 字符串值被 JSON 序列化后的格式。
func (c *HGCodeCache) GetCode(ctx context.Context, phone string) (string, error) {
	if c == nil || c.redisService == nil {
		return "", ErrCodeCacheRedisNil
	}

	loginCodeKey := PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.LoginCodeKey, phone)
	val, err := c.redisService.GetFromRedisV2(loginCodeKey, ctx)
	return decodeRedisCacheStringValue(val), err
}

// DeleteCode 删除短信验证码，调用方可按需记录删除失败。
func (c *HGCodeCache) DeleteCode(ctx context.Context, phone string) error {
	if c == nil || c.redisService == nil {
		return ErrCodeCacheRedisNil
	}

	loginCodeKey := PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.LoginCodeKey, phone)
	return c.redisService.DeleteFromRedis(loginCodeKey, ctx)
}

// decodeRedisCacheStringValue 兼容 SetToRedisV2 写入字符串时被 JSON 序列化的值。
func decodeRedisCacheStringValue(v string) string {
	var result string
	if err := json.Unmarshal([]byte(v), &result); err == nil {
		return result
	}
	return v
}

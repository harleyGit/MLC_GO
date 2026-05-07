/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-07 20:48:42
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-03-01 19:51:11
 * @FilePath: /MLC_GO/internal/modules/user/cache/hg_user_cache.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package cache

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	HGResponsePakcage "MLC_GO/internal/response"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrUserCacheRedisNil 表示用户列表缓存缺少 Redis 依赖。
var ErrUserCacheRedisNil = errors.New("user cache redis dependency is nil")

type HGUserCache struct {
	redisCache *PersistenceRedisPackage.RedisService
}

// NewUserCache 创建用户列表缓存访问对象。
func NewUserCache(redisService *PersistenceRedisPackage.RedisService) *HGUserCache {
	return &HGUserCache{redisCache: redisService}
}

// GetUserListCache 读取用户 cursor 分页缓存；缓存未命中返回 nil, nil。
func (c *HGUserCache) GetUserListCache(
	ctx context.Context,
	cursor int64,
	size int,
) (*HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO], error) {
	if c == nil || c.redisCache == nil {
		return nil, ErrUserCacheRedisNil
	}

	key := fmt.Sprintf(PersistenceRedisPackage.UserListKey, cursor, size)
	val, err := c.redisCache.GetFromRedisV2(key, ctx)
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}

	var resp HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]
	if err := json.Unmarshal([]byte(val), &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// SetUserListCache 写入用户 cursor 分页缓存，key 维度包含 cursor 和 size。
func (c *HGUserCache) SetUserListCache(
	ctx context.Context,
	resp HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO],
	cursor int64,
	size int,
) error {
	if c == nil || c.redisCache == nil {
		return ErrUserCacheRedisNil
	}

	key := fmt.Sprintf(PersistenceRedisPackage.UserListKey, cursor, size)
	expireTime := PersistenceRedisPackage.GetWEBCacheTime()
	return c.redisCache.SetToRedisV2(key, resp, expireTime, ctx)
}

// GetUserListTotalCache 读取用户总数缓存；缓存未命中返回 0, nil。
func (c *HGUserCache) GetUserListTotalCache(ctx context.Context) (int, error) {
	if c == nil || c.redisCache == nil {
		return 0, ErrUserCacheRedisNil
	}

	val, err := c.redisCache.GetFromRedisV2(PersistenceRedisPackage.UserListTotalKey, ctx)
	if err != nil {
		return 0, err
	}
	if val == "" {
		return 0, nil
	}

	var total int
	if err := json.Unmarshal([]byte(val), &total); err != nil {
		return 0, err
	}

	return total, nil
}

// SetUserListTotalCache 写入用户总数缓存。
func (c *HGUserCache) SetUserListTotalCache(ctx context.Context, total int) error {
	if c == nil || c.redisCache == nil {
		return ErrUserCacheRedisNil
	}

	expireTime := PersistenceRedisPackage.GetWEBCacheTime()
	return c.redisCache.SetToRedisV2(PersistenceRedisPackage.UserListTotalKey, total, expireTime, ctx)
}

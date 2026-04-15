/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-07 20:48:42
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-03-01 19:51:11
 * @FilePath: /MLC_GO/internal/modules/user/cache/hg_user_cache.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGUserCachePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	HGResponsePakcage "MLC_GO/internal/response"
	"context"
	"encoding/json"
	"fmt"
)

type HGUserCache struct {
	redisCache *PersistenceRedisPackage.RedisService
}

func NewUserCache(redisService *PersistenceRedisPackage.RedisService) *HGUserCache {
	return &HGUserCache{redisCache: redisService}
}

func (c *HGUserCache) GetUserListCache(
	ctx context.Context,
	cursor int64,
	size int,
) (*HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO], error) {

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

func (c *HGUserCache) SetUserListCache(
	ctx context.Context,
	resp HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO],
	cursor int64,
	size int,
) error {

	// 缓存 key 需要包含 cursor 和 size，避免不同游标页互相覆盖。
	key := fmt.Sprintf(PersistenceRedisPackage.UserListKey, cursor, size)

	// 存：整个分页对象
	expireTime := PersistenceRedisPackage.GetWEBCacheTime()
	err := c.redisCache.SetToRedisV2(key, resp, expireTime, ctx)
	if err != nil {
		return err
	}

	return nil
}

func (c *HGUserCache) GetUserListTotalCache(ctx context.Context) (int, error) {
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

func (c *HGUserCache) SetUserListTotalCache(ctx context.Context, total int) error {
	expireTime := PersistenceRedisPackage.GetWEBCacheTime()
	return c.redisCache.SetToRedisV2(PersistenceRedisPackage.UserListTotalKey, total, expireTime, ctx)
}

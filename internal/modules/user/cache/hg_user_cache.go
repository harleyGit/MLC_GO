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
	"fmt"
)

type HGUserCache struct {
	redisCache *PersistenceRedisPackage.RedisService
}

func NewUserCache(redisService *PersistenceRedisPackage.RedisService) *HGUserCache {
	return &HGUserCache{redisCache: redisService}
}

func (c *HGUserCache) GetUserListCache(ctx context.Context, page, size int) (string, error) {

	key := fmt.Sprintf(PersistenceRedisPackage.UserListKey, page, size)
	val, err := c.redisCache.GetFromRedisV2(key, ctx)
	if err != nil {
		return "", err
	}

	return val, nil
}

func (c *HGUserCache) SetUserListCache(ctx context.Context, resp HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO], page, size int) error {

	// 缓存key应包含page和size
	key := fmt.Sprintf(PersistenceRedisPackage.UserListKey, page, size)

	// 存：整个分页对象
	expireTime := PersistenceRedisPackage.GetWEBCacheTime()
	err := c.redisCache.SetToRedisV2(key, resp, expireTime, ctx)
	if err != nil {
		return err
	}

	return nil
}

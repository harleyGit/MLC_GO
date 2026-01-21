/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 20:11:48
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 20:21:48
 * @FilePath: /MLC_GO/internal/modules/user/cache/hg_code_cache.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserCachePackage

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type HGCodeCache struct {
	rdb *redis.Client
}

func NewCodeCache(rdb *redis.Client) *HGCodeCache {
	return &HGCodeCache{rdb: rdb}
}

func (c *HGCodeCache) SetCode(ctx context.Context, phone, code string) error {
	return c.rdb.Set(
		ctx,
		"login:code:"+phone,
		code,
		5*time.Minute,
	).Err()
}

func (c *HGCodeCache) GetCode(ctx context.Context, phone string) (string, error) {
	return c.rdb.Get(ctx, "login:code:"+phone).Result()
}

func (c *HGCodeCache) DeleteCode(ctx context.Context, phone string) {
	c.rdb.Del(ctx, "login:code:"+phone)
}

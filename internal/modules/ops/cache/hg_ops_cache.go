/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-05-30 22:20:32
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-25 18:46:43
 * @FilePath: /MLC_GO/internal/modules/ops/cache/hg_ops_cache.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package OpsCachePackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache 定义运维管理缓存
type Cache struct {
	redisService *PersistenceRedisPackage.RedisService
	client       *redis.Client
}

// NewCache 创建运维管理缓存实例
func NewCache(redisService *PersistenceRedisPackage.RedisService) *Cache {
	return &Cache{redisService: redisService, client: redisService.Client()}
}

// 缓存键前缀
const (
	RoleCachePrefix       = "ops:role:"
	MenuCachePrefix       = "ops:menu:"
	PermissionCachePrefix = "ops:permission:"
)

const bilibiliActiveTagListTTL = 30 * time.Second

// GetActiveBilibiliTags 读取动画页启用标签缓存。
func (c *Cache) GetActiveBilibiliTags(ctx context.Context) (*OpsDtoPackage.BilibiliTagListResponse, bool, error) {
	// Bytes()将 Redis 返回的字符串 value 转换成 []byte 字节数组，因为redis不理解结构体，存储时需要序列化为 JSON 字符串，读取时需要反序列化为结构体。
	data, err := c.client.Get(ctx, PersistenceRedisPackage.OpsBilibiliActiveTagListKey).Bytes()
	// 判断是否为redis.Nil错误，表示缓存中没有数据。这个要注意与err == redis.Nil的区别，前者是判断错误类型，后者是判断错误值。
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var resp OpsDtoPackage.BilibiliTagListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false, err
	}
	return &resp, true, nil
}

// SetActiveBilibiliTags 写入动画页启用标签短 TTL 缓存，吸收高频读取。
func (c *Cache) SetActiveBilibiliTags(ctx context.Context, resp *OpsDtoPackage.BilibiliTagListResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, PersistenceRedisPackage.OpsBilibiliActiveTagListKey, data, bilibiliActiveTagListTTL).Err()
}

// InvalidateActiveBilibiliTags 在标签写入提交后失效活跃列表缓存。
func (c *Cache) InvalidateActiveBilibiliTags(ctx context.Context) error {
	return c.client.Del(ctx, PersistenceRedisPackage.OpsBilibiliActiveTagListKey).Err()
}

// 缓存过期时间
const (
	RoleCacheExpiration       = 3600 // 1小时
	MenuCacheExpiration       = 3600 // 1小时
	PermissionCacheExpiration = 1800 // 30分钟
)

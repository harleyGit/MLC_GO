package BilibiliCachePackage

import (
	BilibiliDtoPackage "MLC_GO/internal/modules/bilibili/dto"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	hgAuthorProfileTTL = 60 * time.Second
	hgAuthorStatsTTL   = 10 * time.Second
	hgAuthorVideosTTL  = 5 * time.Second
)

// Cache 保存作者公开资料、统计和短周期视频页缓存。
type Cache struct {
	redis *PersistenceRedisPackage.RedisService
}

// NewCache 创建作者空间缓存。
func NewCache(redisService *PersistenceRedisPackage.RedisService) *Cache {
	return &Cache{redis: redisService}
}

func (c *Cache) get(ctx context.Context, key string, target any) (bool, error) {
	if c == nil || c.redis == nil {
		return false, nil
	}
	value, err := c.redis.GetFromRedisV2(key, ctx)
	if err != nil || value == "" {
		return false, err
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Cache) set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if c == nil || c.redis == nil {
		return nil
	}
	return c.redis.SetToRedisV2(key, value, ttl, ctx)
}

// GetProfile 读取作者公开资料缓存。
func (c *Cache) GetProfile(ctx context.Context, userID string) (BilibiliDtoPackage.HGAuthorProfileResponse, bool, error) {
	var value BilibiliDtoPackage.HGAuthorProfileResponse
	hit, err := c.get(ctx, PersistenceRedisPackage.GetBilibiliAuthorProfileKey(userID), &value)
	return value, hit, err
}

// SetProfile 写入作者公开资料缓存。
func (c *Cache) SetProfile(ctx context.Context, userID string, value BilibiliDtoPackage.HGAuthorProfileResponse) error {
	return c.set(ctx, PersistenceRedisPackage.GetBilibiliAuthorProfileKey(userID), value, hgAuthorProfileTTL)
}

// GetStats 读取作者统计缓存。
func (c *Cache) GetStats(ctx context.Context, userID string) (BilibiliDtoPackage.HGAuthorStatsResponse, bool, error) {
	var value BilibiliDtoPackage.HGAuthorStatsResponse
	hit, err := c.get(ctx, PersistenceRedisPackage.GetBilibiliAuthorStatsKey(userID), &value)
	return value, hit, err
}

// SetStats 写入作者统计缓存。
func (c *Cache) SetStats(ctx context.Context, userID string, value BilibiliDtoPackage.HGAuthorStatsResponse) error {
	return c.set(ctx, PersistenceRedisPackage.GetBilibiliAuthorStatsKey(userID), value, hgAuthorStatsTTL)
}

// GetVideos 读取作者视频分页缓存。
func (c *Cache) GetVideos(ctx context.Context, userID, cursor string, pageSize int) (BilibiliDtoPackage.HGAuthorVideoListResponse, bool, error) {
	var value BilibiliDtoPackage.HGAuthorVideoListResponse
	hit, err := c.get(ctx, PersistenceRedisPackage.GetBilibiliAuthorVideosKey(userID, cursor, pageSize), &value)
	return value, hit, err
}

// SetVideos 写入作者视频分页缓存。
func (c *Cache) SetVideos(ctx context.Context, userID, cursor string, pageSize int, value BilibiliDtoPackage.HGAuthorVideoListResponse) error {
	return c.set(ctx, PersistenceRedisPackage.GetBilibiliAuthorVideosKey(userID, cursor, pageSize), value, hgAuthorVideosTTL)
}

// GetFollowerCount 从现有关注计数投影读取粉丝数。
func (c *Cache) GetFollowerCount(ctx context.Context, userID string) (int64, bool, error) {
	if c == nil || c.redis == nil || c.redis.Client() == nil {
		return 0, false, nil
	}
	value, err := c.redis.Client().HGet(ctx, PersistenceRedisPackage.GetUserFollowCountKey(userID), "follow").Result()
	if err != nil {
		return 0, false, err
	}
	count, err := strconv.ParseInt(value, 10, 64)
	return count, err == nil, err
}

// FillVideoCounts 通过单个 Redis Pipeline 批量填充当前页互动计数，避免逐视频网络往返。
func (c *Cache) FillVideoCounts(ctx context.Context, videos []BilibiliDtoPackage.HGAuthorVideoItem) error {
	if c == nil || c.redis == nil || c.redis.Client() == nil || len(videos) == 0 {
		return nil
	}
	pipe := c.redis.Client().Pipeline()
	commands := make([]interface{ Val() []interface{} }, 0, len(videos))
	for i := range videos {
		commands = append(commands, pipe.HMGet(ctx, PersistenceRedisPackage.GetVideoInteractionCountKey(videos[i].SubmissionID), "like", "coin", "favorite", "share"))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return err
	}
	for i, command := range commands {
		values := command.Val()
		videos[i].LikeCount = hgCount(values, 0)
		videos[i].CoinCount = hgCount(values, 1)
		videos[i].FavoriteCount = hgCount(values, 2)
		videos[i].ShareCount = hgCount(values, 3)
	}
	return nil
}

func hgCount(values []interface{}, index int) int64 {
	if index >= len(values) || values[index] == nil {
		return 0
	}
	value, err := strconv.ParseInt(fmt.Sprint(values[index]), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

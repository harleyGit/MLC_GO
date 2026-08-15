package VideoRecommendCachePackage

import (
	VideoRecommendDtoPackage "MLC_GO/internal/modules/video_recommend/dto"
	VideoRecommendModelPackage "MLC_GO/internal/modules/video_recommend/model"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// hgCardCacheTTL 是视频公开卡片的基础 TTL；实际写入时附加稳定抖动，避免同批 key 集中过期。
	hgCardCacheTTL = 10 * time.Minute
	// hgShardOverread 为每个固定分片保留少量过采样空间，用于抵消同分值边界过滤和失效候选。
	hgShardOverread = 2
)

// Cache 从分片 Feed 召回候选，并批量维护卡片缓存和互动计数。
type Cache struct {
	redis      *PersistenceRedisPackage.RedisService
	generation string
	shardCount int
}

// NewCache 创建推荐缓存读侧；generation 与 shardCount 必须和 Kafka Feed 投影一致。
func NewCache(redisService *PersistenceRedisPackage.RedisService, generation string, shardCount int) *Cache {
	return &Cache{redis: redisService, generation: generation, shardCount: shardCount}
}

// ListCandidates 通过单个 Pipeline 读取全部固定分片的小窗口，并在进程内执行有界归并。
func (c *Cache) ListCandidates(ctx context.Context, cursor VideoRecommendModelPackage.HGCursor, limit int) ([]VideoRecommendModelPackage.HGCandidate, error) {
	if c == nil || c.redis == nil || c.redis.Client() == nil {
		return nil, fmt.Errorf("video recommend redis cannot be nil")
	}
	if limit <= 0 || c.shardCount <= 0 {
		return []VideoRecommendModelPackage.HGCandidate{}, nil
	}
	maxScore := "+inf"
	if cursor.Score > 0 {
		// Redis 的 score 上界是闭区间，同分值候选再由 submission_id 做严格复合游标过滤。
		maxScore = strconv.FormatInt(cursor.Score, 10)
	}
	window := int64(limit*hgShardOverread + 1)
	pipe := c.redis.Client().Pipeline()
	commands := make([]*redis.ZSliceCmd, c.shardCount)
	for shard := 0; shard < c.shardCount; shard++ {
		commands[shard] = pipe.ZRevRangeByScoreWithScores(ctx, PersistenceRedisPackage.GetFeedPublishedZSetKey(c.generation, shard), &redis.ZRangeBy{Max: maxScore, Min: "-inf", Offset: 0, Count: window})
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("read video recommend feed shards: %w", err)
	}
	candidates := make([]VideoRecommendModelPackage.HGCandidate, 0, c.shardCount*limit)
	seen := make(map[string]struct{}, c.shardCount*limit)
	for _, command := range commands {
		for _, value := range command.Val() {
			submissionID := fmt.Sprint(value.Member)
			score := int64(value.Score)
			if cursor.Score > 0 && (score > cursor.Score || (score == cursor.Score && submissionID >= cursor.SubmissionID)) {
				continue
			}
			if _, ok := seen[submissionID]; ok {
				continue
			}
			seen[submissionID] = struct{}{}
			candidates = append(candidates, VideoRecommendModelPackage.HGCandidate{SubmissionID: submissionID, Score: score})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].SubmissionID > candidates[j].SubmissionID
		}
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

// GetCards 使用 MGET 批量读取视频卡片缓存，返回命中项和冷缺失 ID。
func (c *Cache) GetCards(ctx context.Context, submissionIDs []string) (map[string]VideoRecommendDtoPackage.HGFeedItem, []string, error) {
	items := make(map[string]VideoRecommendDtoPackage.HGFeedItem, len(submissionIDs))
	if len(submissionIDs) == 0 {
		return items, nil, nil
	}
	keys := make([]string, len(submissionIDs))
	for i, id := range submissionIDs {
		keys[i] = PersistenceRedisPackage.GetVideoRecommendCardKey(id)
	}
	values, err := c.redis.Client().MGet(ctx, keys...).Result()
	if err != nil && err != redis.Nil {
		return nil, nil, fmt.Errorf("read video recommend card cache: %w", err)
	}
	misses := make([]string, 0, len(submissionIDs))
	for i, value := range values {
		if value == nil {
			misses = append(misses, submissionIDs[i])
			continue
		}
		var item VideoRecommendDtoPackage.HGFeedItem
		if err := json.Unmarshal([]byte(fmt.Sprint(value)), &item); err != nil {
			misses = append(misses, submissionIDs[i])
			continue
		}
		items[item.SubmissionID] = item
	}
	return items, misses, nil
}

// SetCards 使用 Pipeline 写入有抖动的卡片 TTL，降低同一时刻大面积过期风险。
func (c *Cache) SetCards(ctx context.Context, items map[string]VideoRecommendDtoPackage.HGFeedItem) error {
	if len(items) == 0 {
		return nil
	}
	pipe := c.redis.Client().Pipeline()
	for id, item := range items {
		value, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("marshal video recommend card: %w", err)
		}
		jitter := time.Duration(PersistenceRedisPackage.GetFeedShard(id, 120)) * time.Second
		pipe.Set(ctx, PersistenceRedisPackage.GetVideoRecommendCardKey(id), value, hgCardCacheTTL+jitter)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("write video recommend card cache: %w", err)
	}
	return nil
}

// FillInteractionCounts 使用单个 Pipeline 批量填充当前页互动计数。
func (c *Cache) FillInteractionCounts(ctx context.Context, items []VideoRecommendDtoPackage.HGFeedItem) error {
	if len(items) == 0 {
		return nil
	}
	pipe := c.redis.Client().Pipeline()
	commands := make([]*redis.SliceCmd, len(items))
	for i := range items {
		commands[i] = pipe.HMGet(ctx, PersistenceRedisPackage.GetVideoInteractionCountKey(items[i].SubmissionID), "like", "coin", "favorite", "share")
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return fmt.Errorf("read video recommend interaction counts: %w", err)
	}
	for i, command := range commands {
		values := command.Val()
		items[i].LikeCount = hgCount(values, 0)
		items[i].CoinCount = hgCount(values, 1)
		items[i].FavoriteCount = hgCount(values, 2)
		items[i].ShareCount = hgCount(values, 3)
	}
	return nil
}

func hgCount(values []interface{}, index int) int64 {
	if index >= len(values) || values[index] == nil {
		return 0
	}
	value, _ := strconv.ParseInt(fmt.Sprint(values[index]), 10, 64)
	return value
}

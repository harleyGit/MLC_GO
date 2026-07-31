package VideoInteractionCachePackage

import (
	VideoInteractionDtoPackage "MLC_GO/internal/modules/video_interaction/dto"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// Cache 保存详情页实时互动状态；Kafka Consumer 会把 MySQL 最终状态重新投影到这些 key。
type Cache struct {
	redis *PersistenceRedisPackage.RedisService
}

func NewCache(redisService *PersistenceRedisPackage.RedisService) *Cache {
	return &Cache{redis: redisService}
}

func (c *Cache) GetState(ctx context.Context, userID string, submissionID string, authorID string) (VideoInteractionDtoPackage.StateResponse, error) {
	response := VideoInteractionDtoPackage.StateResponse{SubmissionID: submissionID, AuthorID: authorID}
	client := c.redis.Client()
	pipe := client.Pipeline()
	stateCmd := pipe.HMGet(ctx, PersistenceRedisPackage.GetVideoInteractionStateKey(userID, submissionID), "like", "favorite", "coin")
	countCmd := pipe.HMGet(ctx, PersistenceRedisPackage.GetVideoInteractionCountKey(submissionID), "like", "favorite", "coin", "share")
	var followCmd *redis.StringCmd
	var followerCountCmd *redis.StringCmd
	if authorID != "" {
		followCmd = pipe.HGet(ctx, PersistenceRedisPackage.GetUserFollowStateKey(userID, authorID), "follow")
		followerCountCmd = pipe.HGet(ctx, PersistenceRedisPackage.GetUserFollowCountKey(authorID), "follow")
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return response, err
	}
	state := stateCmd.Val()
	counts := countCmd.Val()
	response.Liked = hgBool(state, 0)
	response.Favorited = hgBool(state, 1)
	response.UserCoinCount = hgInt64(state, 2)
	response.LikeCount = hgInt64(counts, 0)
	response.FavoriteCount = hgInt64(counts, 1)
	response.CoinCount = hgInt64(counts, 2)
	response.ShareCount = hgInt64(counts, 3)
	if followCmd != nil {
		response.Followed = followCmd.Val() == "1"
		response.FollowerCount, _ = followerCountCmd.Int64()
	}
	return response, nil
}

func (c *Cache) ApplyOptimistic(ctx context.Context, userID string, submissionID string, targetID string, action string, active bool, quantity int) error {
	stateKey := PersistenceRedisPackage.GetVideoInteractionStateKey(userID, submissionID)
	countKey := PersistenceRedisPackage.GetVideoInteractionCountKey(submissionID)
	if action == "follow" {
		stateKey = PersistenceRedisPackage.GetUserFollowStateKey(userID, targetID)
		countKey = PersistenceRedisPackage.GetUserFollowCountKey(targetID)
	}
	activeValue := "0"
	if active {
		activeValue = "1"
	}
	return c.redis.Eval(ctx, PersistenceRedisPackage.VideoInteractionOptimisticLuaScript, []string{stateKey, countKey}, action, activeValue, quantity)
}

func hgBool(values []any, index int) bool {
	return index < len(values) && values[index] != nil && values[index].(string) == "1"
}

func hgInt64(values []any, index int) int64 {
	if index >= len(values) || values[index] == nil {
		return 0
	}
	value, _ := strconv.ParseInt(values[index].(string), 10, 64)
	return value
}

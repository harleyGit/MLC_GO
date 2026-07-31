package VideoInteractionCachePackage

import (
	VideoInteractionRepositoryPackage "MLC_GO/internal/modules/video_interaction/repository"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrHGProjectionLeaseLost 表示当前 worker 的租约已过期或被其他副本替换，禁止继续推进 checkpoint。
var ErrHGProjectionLeaseLost = errors.New("interaction projection lease lost")

// AcquireLease 使用 SET NX PX 和随机 token 为单条 stream 建立跨副本所有权。
func (c *Cache) AcquireLease(ctx context.Context, stream string, ttl time.Duration) (string, bool, error) {
	var tokenBytes [16]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return "", false, fmt.Errorf("generate projection lease token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes[:])
	acquired, err := c.redis.Client().SetNX(ctx, PersistenceRedisPackage.GetInteractionReprojectLeaseKey(stream), token, ttl).Result()
	if err != nil {
		return "", false, fmt.Errorf("acquire projection lease: %w", err)
	}
	return token, acquired, nil
}

func (c *Cache) LoadCheckpoint(ctx context.Context, stream string) (string, error) {
	checkpoint, err := c.redis.Client().Get(ctx, PersistenceRedisPackage.GetInteractionReprojectCheckpointKey(stream)).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load projection checkpoint: %w", err)
	}
	return checkpoint, nil
}

// CommitCheckpoint 原子校验 owner token、写入游标并释放 lease，防止过期 worker 覆盖新 owner 的进度。
func (c *Cache) CommitCheckpoint(ctx context.Context, stream string, token string, checkpoint string) error {
	result, err := c.redis.Client().Eval(ctx, PersistenceRedisPackage.InteractionReprojectCommitLuaScript,
		[]string{PersistenceRedisPackage.GetInteractionReprojectLeaseKey(stream), PersistenceRedisPackage.GetInteractionReprojectCheckpointKey(stream)}, token, checkpoint).Int()
	if err != nil {
		return fmt.Errorf("commit projection checkpoint: %w", err)
	}
	if result != 1 {
		return ErrHGProjectionLeaseLost
	}
	return nil
}

func (c *Cache) ReleaseLease(ctx context.Context, stream string, token string) error {
	if err := c.redis.Client().Eval(ctx, PersistenceRedisPackage.InteractionReprojectReleaseLuaScript,
		[]string{PersistenceRedisPackage.GetInteractionReprojectLeaseKey(stream)}, token).Err(); err != nil {
		return fmt.Errorf("release projection lease: %w", err)
	}
	return nil
}

// ApplyVideoStates 在有界 pipeline 中写绝对状态；重放不会像 HINCRBY 一样重复累计。
func (c *Cache) ApplyVideoStates(ctx context.Context, rows []VideoInteractionRepositoryPackage.HGVideoStateProjection) error {
	pipe := c.redis.Client().Pipeline()
	for _, row := range rows {
		value := int64(0)
		if row.Active {
			value = 1
		}
		if row.InteractionType == "coin" {
			value = row.Quantity
		}
		pipe.HSet(ctx, PersistenceRedisPackage.GetVideoInteractionStateKey(row.UserID, row.SubmissionID), row.InteractionType, value)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) ApplyFollowStates(ctx context.Context, rows []VideoInteractionRepositoryPackage.HGFollowStateProjection) error {
	pipe := c.redis.Client().Pipeline()
	for _, row := range rows {
		value := 0
		if row.Active {
			value = 1
		}
		pipe.HSet(ctx, PersistenceRedisPackage.GetUserFollowStateKey(row.FollowerID, row.FolloweeID), "follow", value)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) ApplyVideoCounts(ctx context.Context, rows []VideoInteractionRepositoryPackage.HGVideoCountProjection) error {
	pipe := c.redis.Client().Pipeline()
	for _, row := range rows {
		pipe.HSet(ctx, PersistenceRedisPackage.GetVideoInteractionCountKey(row.SubmissionID), map[string]any{
			"like": row.LikeCount, "coin": row.CoinCount, "favorite": row.FavoriteCount, "share": row.ShareCount,
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) ApplyFollowCounts(ctx context.Context, rows []VideoInteractionRepositoryPackage.HGFollowCountProjection) error {
	pipe := c.redis.Client().Pipeline()
	for _, row := range rows {
		pipe.HSet(ctx, PersistenceRedisPackage.GetUserFollowCountKey(row.UserID), "follow", row.FollowerCount)
	}
	_, err := pipe.Exec(ctx)
	return err
}

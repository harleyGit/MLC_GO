package lease

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// HGTaskLease coordinates one crawler task execution across application instances.
type HGTaskLease interface {
	Acquire(ctx context.Context, taskID uint64, ttl time.Duration) (token string, acquired bool, err error)
	Release(ctx context.Context, taskID uint64, token string) error
}

// HGRedisTaskLease uses Redis SET NX TTL and token-checked Lua release for per-task ownership.
type HGRedisTaskLease struct {
	redis *PersistenceRedisPackage.RedisService
}

// NewHGRedisTaskLease reuses the application Redis pool.
func NewHGRedisTaskLease(redisService *PersistenceRedisPackage.RedisService) *HGRedisTaskLease {
	return &HGRedisTaskLease{redis: redisService}
}

// Acquire attempts to own one task for a bounded execution interval.
func (l *HGRedisTaskLease) Acquire(ctx context.Context, taskID uint64, ttl time.Duration) (string, bool, error) {
	if l == nil || l.redis == nil || l.redis.Client() == nil || taskID == 0 || ttl <= 0 {
		return "", false, errors.New("crawler task lease dependencies are invalid")
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", false, fmt.Errorf("generate crawler task lease token: %w", err)
	}
	token := hex.EncodeToString(raw[:])
	acquired, err := l.redis.Client().SetNX(ctx, PersistenceRedisPackage.GetCrawlerTaskLeaseKey(taskID), token, ttl).Result()
	if err != nil {
		return "", false, fmt.Errorf("acquire crawler task lease: %w", err)
	}
	return token, acquired, nil
}

// Release deletes the lease only while token still identifies its current owner.
func (l *HGRedisTaskLease) Release(ctx context.Context, taskID uint64, token string) error {
	if l == nil || l.redis == nil || taskID == 0 || token == "" {
		return errors.New("crawler task lease release input is invalid")
	}
	if err := l.redis.Eval(ctx, PersistenceRedisPackage.ReleaseSubmitLockLuaScript, []string{PersistenceRedisPackage.GetCrawlerTaskLeaseKey(taskID)}, token); err != nil {
		return fmt.Errorf("release crawler task lease: %w", err)
	}
	return nil
}

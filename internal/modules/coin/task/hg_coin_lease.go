package CoinTaskPackage

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// HGRedisJobLease 使用 Redis SET NX TTL 为每轮硬币维护任务选举一个应用副本。
// value 是随机 token，释放时通过 compare-and-delete Lua 校验 owner，避免旧 worker 删除新 owner 的 lease。
type HGRedisJobLease struct {
	redis *PersistenceRedisPackage.RedisService
}

// NewHGRedisJobLease 复用应用级 Redis 连接池，不为后台任务额外创建连接池。
func NewHGRedisJobLease(redisService *PersistenceRedisPackage.RedisService) *HGRedisJobLease {
	return &HGRedisJobLease{redis: redisService}
}

// Acquire 尝试获取有 TTL 的单轮所有权；未获得时调用方不会访问 MySQL。
func (l *HGRedisJobLease) Acquire(ctx context.Context, ttl time.Duration) (string, bool, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", false, fmt.Errorf("generate coin job lease token: %w", err)
	}
	token := hex.EncodeToString(raw[:])
	acquired, err := l.redis.Client().SetNX(ctx, PersistenceRedisPackage.CoinJobLeaseKey, token, ttl).Result()
	return token, acquired, err
}

// Release 仅在 token 仍匹配当前 owner 时删除 lease。
func (l *HGRedisJobLease) Release(ctx context.Context, token string) error {
	return l.redis.Eval(ctx, PersistenceRedisPackage.ReleaseSubmitLockLuaScript, []string{PersistenceRedisPackage.CoinJobLeaseKey}, token)
}

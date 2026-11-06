package UserServicePackage

import (
	userrepository "MLC_GO/internal/modules/user/repository"
	"MLC_GO/internal/pkg/logHG"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// HGErrUserDeletionInProgress 表示已有删除隔离标记，不能重入。
var HGErrUserDeletionInProgress = errors.New("该用户删除中或已删除")

// HGErrUserDeleteSelf 表示目标账号就是操作者，避免大小写不敏感 SQL 绕过本人保护。
var HGErrUserDeleteSelf = errors.New("不能删除当前登录账号")

// HGErrUserDeleteBusy 表示删除校验并发已满，避免大量跨表点查挤占普通请求连接池。
var HGErrUserDeleteBusy = errors.New("删除请求过多，请稍后重试")

// hgUserDeleteSlots 对单服务实例的重型删除校验限并发；跨实例仍由认证隔离 key 串行同一账号。
var hgUserDeleteSlots = make(chan struct{}, 4)

// hgUserDeleteFenceTTL 覆盖当前配置的所有刷新令牌有效期，另外保留一天缓冲。
func hgUserDeleteFenceTTL() time.Duration { return RefreshTTL + 24*time.Hour }

// HGUserDeleted 校验账号删除隔离标记；Redis 不可用必须阻止认证。
func HGUserDeleted(ctx context.Context, rdb *redis.Client, userID string) (bool, error) {
	if rdb == nil || userID == "" {
		return false, errors.New("用户隔离校验依赖不可用")
	}
	n, err := rdb.Exists(ctx, PersistenceRedisPackage.HGUserDeleteFenceKey(userID)).Result()
	return n != 0, err
}

// HGDeleteUnusedUser 先隔离账号认证，再事务删除无关联账号，提交后有界撤销旧会话。
func (s *UserService) HGDeleteUnusedUser(ctx context.Context, userID, actorID string) error {
	if userID == "" || actorID == "" || s == nil || s.repo == nil || s.redisService == nil || s.redisService.Client() == nil {
		return errors.New("用户删除依赖不可用")
	}
	select {
	case hgUserDeleteSlots <- struct{}{}:
		defer func() { <-hgUserDeleteSlots }()
	default:
		return HGErrUserDeleteBusy
	}
	rdb := s.redisService.Client()
	canonicalID, err := s.repo.HGCanonicalDeleteUserID(ctx, userID)
	if err != nil {
		return err
	}
	if canonicalID == actorID {
		return HGErrUserDeleteSelf
	}
	userID = canonicalID
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	key := PersistenceRedisPackage.HGUserDeleteFenceKey(userID)
	set, err := rdb.SetNX(checkCtx, key, "1", hgUserDeleteFenceTTL()).Result()
	if err != nil {
		return fmt.Errorf("隔离账号认证失败: %w", err)
	}
	if !set {
		return HGErrUserDeletionInProgress
	}
	err = s.repo.HGDeleteUnusedUser(ctx, userID, actorID)
	if err != nil {
		if !errors.Is(err, userrepository.HGErrUserDeleteCommitUncertain) {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
			if cleanupErr := rdb.Del(cleanupCtx, key).Err(); cleanupErr != nil {
				logHG.ErrFInfo("用户删除失败后清理认证隔离标记失败: %v", cleanupErr)
				cleanupCancel()
				return fmt.Errorf("认证隔离标记清理失败，请核查账号状态: %w", cleanupErr)
			}
			cleanupCancel()
		}
		return err
	}
	// 提交已成功；Redis 清理失败时保留隔离标记，覆盖所有未到期 JWT。
	if err := hgRevokeDeletedUserTokens(rdb, userID); err != nil {
		logHG.ErrFInfo("用户删除成功，但旧令牌清理失败（隔离标记仍有效）: %v", err)
	}
	s.clearUserListCache(ctx)
	return nil
}

// hgRevokeDeletedUserTokens 分批清理该账号的全部登录会话、access 与 refresh 状态。
func hgRevokeDeletedUserTokens(rdb *redis.Client, userID string) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, prefix := range []string{PersistenceRedisPackage.HGAuthSessionKey, PersistenceRedisPackage.AuthTokenKey, PersistenceRedisPackage.AuthRefreshKey} {
		var cursor uint64
		for {
			keys, next, err := rdb.Scan(cleanupCtx, cursor, prefix+hgEscapeRedisGlob(userID)+":*", 100).Result()
			if err != nil {
				return err
			}
			if len(keys) > 0 {
				if err := rdb.Del(cleanupCtx, keys...).Err(); err != nil {
					return err
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	return nil
}

// hgEscapeRedisGlob 防止历史用户 ID 中的通配符扩大 SCAN 匹配范围并误撤销其他账号。
func hgEscapeRedisGlob(userID string) string {
	return strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]").Replace(userID)
}

// HGUserDeleteIsNotFound 判断删除时用户行不存在，供 HTTP 层映射 404。
func HGUserDeleteIsNotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }

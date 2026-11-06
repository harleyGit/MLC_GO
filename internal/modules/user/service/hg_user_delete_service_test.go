package UserServicePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type hgDeleteTokenHook struct {
	patterns []string
	deleted  []string
	fenced   bool
}

func (*hgDeleteTokenHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (*hgDeleteTokenHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (hook *hgDeleteTokenHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		switch cmd.Name() {
		case "exists":
			if hook.fenced {
				cmd.(*redis.IntCmd).SetVal(1)
			} else {
				cmd.(*redis.IntCmd).SetVal(0)
			}
		case "scan":
			pattern := cmd.Args()[3].(string)
			hook.patterns = append(hook.patterns, pattern)
			if strings.HasPrefix(pattern, PersistenceRedisPackage.HGAuthSessionKey) {
				cmd.(*redis.ScanCmd).SetVal([]string{PersistenceRedisPackage.HGAuthSessionKey + "a*b:session"}, 0)
			} else {
				cmd.(*redis.ScanCmd).SetVal(nil, 0)
			}
		case "del":
			hook.deleted = append(hook.deleted, cmd.Args()[1].(string))
			cmd.(*redis.IntCmd).SetVal(1)
		default:
			return errors.New("未预期 Redis 操作")
		}
		return nil
	}
}

func TestHGEscapeRedisGlob(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"normal_user-1", "normal_user-1"},
		{"a*b?c[d]\\e", `a\*b\?c\[d\]\\e`},
	} {
		if got := hgEscapeRedisGlob(tc.input); got != tc.want {
			t.Fatalf("SCAN 用户 ID %q 未正确转义: %q", tc.input, got)
		}
	}
}

func TestHGUserDeleteFenceFailsClosed(t *testing.T) {
	if _, err := HGUserDeleted(context.Background(), nil, "user-1"); err == nil {
		t.Fatal("Redis 不可用时不得允许认证")
	}
	if hgUserDeleteFenceTTL() <= RefreshTTL || hgUserDeleteFenceTTL() <= 24*time.Hour {
		t.Fatal("删除隔离应覆盖旧刷新令牌有效期")
	}
}

func TestHGUserDeletedChecksFence(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer rdb.Close()
	hook := &hgDeleteTokenHook{fenced: true}
	rdb.AddHook(hook)
	if deleted, err := HGUserDeleted(context.Background(), rdb, "user-1"); err != nil || !deleted {
		t.Fatalf("已删除账号不能被认证: deleted=%v err=%v", deleted, err)
	}
	hook.fenced = false
	if deleted, err := HGUserDeleted(context.Background(), rdb, "user-1"); err != nil || deleted {
		t.Fatalf("正常账号认证不应被误阻断: deleted=%v err=%v", deleted, err)
	}
}

func TestHGRevokeDeletedUserTokensEscapesPattern(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer rdb.Close()
	hook := &hgDeleteTokenHook{}
	rdb.AddHook(hook)
	if err := hgRevokeDeletedUserTokens(rdb, "a*b"); err != nil {
		t.Fatal(err)
	}
	if len(hook.patterns) != 3 || hook.patterns[0] != PersistenceRedisPackage.HGAuthSessionKey+`a\*b:*` || len(hook.deleted) != 1 || hook.deleted[0] != PersistenceRedisPackage.HGAuthSessionKey+"a*b:session" {
		t.Fatalf("撤销令牌的用户范围不正确: pattern=%v deleted=%v", hook.patterns, hook.deleted)
	}
}

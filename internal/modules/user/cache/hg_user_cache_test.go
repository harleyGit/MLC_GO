package cache

import (
	"context"
	"errors"
	"testing"

	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	HGResponsePakcage "MLC_GO/internal/response"
)

func TestUserCache_NilRedisDependency(t *testing.T) {
	c := NewUserCache(nil)
	ctx := context.Background()
	resp := HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]{}

	if _, err := c.GetUserListCache(ctx, 0, 20); !errors.Is(err, ErrUserCacheRedisNil) {
		t.Fatalf("GetUserListCache err=%v, want ErrUserCacheRedisNil", err)
	}
	if err := c.SetUserListCache(ctx, resp, 0, 20); !errors.Is(err, ErrUserCacheRedisNil) {
		t.Fatalf("SetUserListCache err=%v, want ErrUserCacheRedisNil", err)
	}
	if _, err := c.GetUserListTotalCache(ctx); !errors.Is(err, ErrUserCacheRedisNil) {
		t.Fatalf("GetUserListTotalCache err=%v, want ErrUserCacheRedisNil", err)
	}
	if err := c.SetUserListTotalCache(ctx, 10); !errors.Is(err, ErrUserCacheRedisNil) {
		t.Fatalf("SetUserListTotalCache err=%v, want ErrUserCacheRedisNil", err)
	}
}

func TestUserListCacheHasProfileFields(t *testing.T) {
	gender := 0
	if hgUserListCacheHasProfileFields([]*UserDtoPackage.HGCreateUserDTO{{}}) {
		t.Fatal("旧缓存缺少资料列时应回源")
	}
	if !hgUserListCacheHasProfileFields([]*UserDtoPackage.HGCreateUserDTO{{Gender: &gender}}) {
		t.Fatal("新缓存允许头像为空，但需要 gender 字段")
	}
	if !hgUserListCacheHasProfileFields(nil) {
		t.Fatal("空列表缓存无需回源")
	}
}

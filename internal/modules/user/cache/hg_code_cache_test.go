package cache

import (
	"context"
	"errors"
	"testing"
)

func TestDecodeRedisCacheStringValue(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "json string", raw: `"123456"`, want: "123456"},
		{name: "raw string", raw: "123456", want: "123456"},
		{name: "invalid json", raw: "{invalid-json", want: "{invalid-json"},
		{name: "empty", raw: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeRedisCacheStringValue(tt.raw); got != tt.want {
				t.Fatalf("decodeRedisCacheStringValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCodeCache_NilRedisDependency(t *testing.T) {
	c := NewCodeCache(nil)
	ctx := context.Background()

	if err := c.SetCode(ctx, "13800000000", "123456"); !errors.Is(err, ErrCodeCacheRedisNil) {
		t.Fatalf("SetCode err=%v, want ErrCodeCacheRedisNil", err)
	}
	if _, err := c.GetCode(ctx, "13800000000"); !errors.Is(err, ErrCodeCacheRedisNil) {
		t.Fatalf("GetCode err=%v, want ErrCodeCacheRedisNil", err)
	}
	if err := c.DeleteCode(ctx, "13800000000"); !errors.Is(err, ErrCodeCacheRedisNil) {
		t.Fatalf("DeleteCode err=%v, want ErrCodeCacheRedisNil", err)
	}
	if err := c.SaveMultiportConcrolCache(ctx, 1, "ios", "jti", 0); !errors.Is(err, ErrCodeCacheRedisNil) {
		t.Fatalf("SaveMultiportConcrolCache err=%v, want ErrCodeCacheRedisNil", err)
	}
	if _, err := c.GetMultiportConcrolCache(ctx, 1, "ios"); !errors.Is(err, ErrCodeCacheRedisNil) {
		t.Fatalf("GetMultiportConcrolCache err=%v, want ErrCodeCacheRedisNil", err)
	}
}

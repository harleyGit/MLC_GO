package OpsCachePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
)

// Cache 定义运维管理缓存
type Cache struct {
	redisService *PersistenceRedisPackage.RedisService
}

// NewCache 创建运维管理缓存实例
func NewCache(redisService *PersistenceRedisPackage.RedisService) *Cache {
	return &Cache{redisService: redisService}
}

// 缓存键前缀
const (
	RoleCachePrefix     = "ops:role:"
	MenuCachePrefix     = "ops:menu:"
	PermissionCachePrefix = "ops:permission:"
)

// 缓存过期时间
const (
	RoleCacheExpiration     = 3600 // 1小时
	MenuCacheExpiration     = 3600 // 1小时
	PermissionCacheExpiration = 1800 // 30分钟
)
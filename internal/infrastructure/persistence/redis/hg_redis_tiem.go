package PersistenceRedisPackage

import (
	"math/rand"
	"time"
)

// 缓存策略配置【根据实际调整】
var CacheTimeConfig = struct {
	MaxPageToCache int           // 最大可缓存的页码（超过则不缓存）
	MaxSizeToCache int           // 最大每页数量（超过则不缓存）
	HotPageTTL     time.Duration // 热门页的TTL（秒）,比如第1页；注意：访问频率越高要缓存更久，最大化减轻数据库的压力
	WarmPageTTL    time.Duration // 温页的TTL（秒），比如第2-10页
	ColdPageTTL    time.Duration // 冷页的TTL（秒），比如第10-100页及以后
	JitterRangeSec int           // TTL抖动范围（秒），用于随机化过期时间，避免缓存雪崩
}{
	MaxPageToCache: 100,
	MaxSizeToCache: 100,
	HotPageTTL:     30 * time.Minute, // 第1页：30分钟
	WarmPageTTL:    10 * time.Minute, // 第2～10页：10分钟
	ColdPageTTL:    2 * time.Minute,  // 第11～100页：2分钟
	JitterRangeSec: 120,              // 抖动范围±0~±120秒随机
}

// AddJitter 为基础 TTL 添加随机抖动，防止缓存雪崩
// jitterRangeSec: 随机增加的秒数上限（例如 120 表示 +0~120 秒）
// 返回 baseTTL + [0, jitterRangeSec) 秒
func AddJitter(baseTTL time.Duration, jitterRangeSec int) time.Duration {
	if jitterRangeSec <= 0 {
		return baseTTL
	}
	jitterSec := rand.Intn(jitterRangeSec)
	return baseTTL + time.Duration(jitterSec)*time.Second
}

// AddJitterInRange 在 [minTTL, maxTTL) 范围内返回一个随机 TTL
// 更适合需要严格上下限的场景
func AddJitterInRange(minTTL, maxTTL time.Duration) time.Duration {
	if minTTL >= maxTTL {
		return minTTL

	}
	delta := maxTTL - minTTL
	randomDelta := time.Duration(rand.Int63n(int64(delta)))
	return minTTL + randomDelta
}

/*
| 通用 Web 应用 | 5 ~ 30 分钟 | 平衡一致性与性能 |·

普通缓存（如用户详情），基础 TTL = 20 分钟，±15 分钟抖动
*/
func GetWEBCacheTime() time.Duration {
	baseTTL := 20 * time.Minute // 基础5分钟
	rangeSec := 15 * 60         // 抖动范围15分钟
	ttl := AddJitter(baseTTL, rangeSec)
	return ttl
}

/* | 高实时性要求 | 10 ~ 60 秒 | 如后台管理系统，希望尽快看到新用户 | */
func GetIMCacheTime() time.Duration {

	baseTTL := 30 * time.Second // 基础30秒
	rangeSec := 20              // 抖动范围20秒
	ttl := AddJitter(baseTTL, rangeSec)
	return ttl
}

/* | 低频变更 + 高负载 | 30 分钟 ~ 2 小时 | 如公开用户目录，数据稳定 | */
func GetLowFrequencyANDHighLoadCacheTime() time.Duration {
	baseTTL := 90 * time.Minute // 基础1.5小时
	rangeSec := 60 * 60         // 抖动范围1小时
	ttl := AddJitter(baseTTL, rangeSec)
	return ttl
}

/*
GetCacheTimeTTL 根据page和size返回合适的缓存过期时间
返回 0 表示不应该缓存（如page/size超限制）
*/
func GetCacheTimeTTL(page, size int) time.Duration {
	// 1.拒绝不合理或高开销的请求缓存
	if page <= 0 || size <= 10 {
		return 0
	}

	if page > CacheTimeConfig.MaxPageToCache {
		return 0 // 超过最大页码限制，不缓存
	}
	if size > CacheTimeConfig.MaxSizeToCache {
		return 0 // 单页过大，不缓存（防止内存爆炸）
	}

	// 2.根据page判断热度，选择基础TTL
	var baseTTL time.Duration
	switch {
	case page == 1:
		baseTTL = CacheTimeConfig.HotPageTTL // 第1页：最长TTL
	case page <= 10:
		baseTTL = CacheTimeConfig.WarmPageTTL // 第2～10页：中等TTL
	case page <= CacheTimeConfig.MaxPageToCache:
		baseTTL = CacheTimeConfig.ColdPageTTL // 第11～100页：较短TTL
	default:
		return 0 // 超过最大页码限制，不缓存
	}

	// 3.添加随机抖动，避免缓存雪崩
	if CacheTimeConfig.JitterRangeSec > 0 {

		jitterSec := rand.Intn(CacheTimeConfig.JitterRangeSec)
		baseTTL += time.Duration(jitterSec) * time.Second
	}
	return baseTTL
}

/*使用举例：
// 在你的缓存设置逻辑中
ttl := cache.GetCacheTimeTTL(page, size)
if ttl > 0 {
    key := fmt.Sprintf("user:list:page:%d:size:%d", page, size)
    err := c.redisCache.SetToRedis(key, resp, ttl, ctx)
    if err != nil {
        // 记录日志，但不要中断主流程
        log.Printf("Failed to cache user list: %v", err)
    }
}

| 条件 | 是否缓存 | TTL（示例） | 说明 |
|------|--------|------------|------|
| `page=1, size=10` | ✅ | 30m ± 0~120s | 首页，最高热度 |
| `page=5, size=20` | ✅ | 10m ± 0~120s | 常用页 |
| `page=50, size=10` | ✅ | 2m ± 0~120s | 冷门页，短缓存 |
| `page=200, size=10` | ❌ | 0 | 超出 MaxPageToCache |
| `page=1, size=500` | ❌ | 0 | 超出 MaxSizeToCache |
| `page=-1` 或 `size=0` | ❌ | 0 | 非法参数 |

*/

/*
使用示例
场景 1：普通缓存（如用户详情），基础 TTL = 10 分钟，±2 分钟抖动
baseTTL := 10 * time.Minute
ttl := cache.AddJitter(baseTTL, 120) // 120 秒 = 2 分钟
// 结果：10m ~ 12m 之间


场景 2：严格控制在 9~11 分钟之间
ttl := cache.AddJitterInRange(9*time.Minute, 11*time.Minute)

场景 3：集成到你的 Redis Set 方法中
func (c *Cache) SetUser(ctx context.Context, userID string, user *User) error {
    key := "user:" + userID
    ttl := cache.AddJitter(15*time.Minute, 180) // 15~18 分钟
    return c.redis.Set(key, user, ttl)
}
*/

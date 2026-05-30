/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-22 14:26:12
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-22 14:26:16
 * @FilePath: /MLC_GO/internal/pkg/redis/hg_redis_script.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package PersistenceRedisPackage

const (
	// TokenBucketRateLimitLuaScript 是令牌桶限流脚本，用于替代固定窗口 INCR + EXPIRE。
	// 固定窗口的问题：例如每分钟限制 120 次，用户可能在 00:59 打 120 次、01:00 再打 120 次，瞬间形成 240 次临界突刺。
	// 令牌桶按时间连续补充 token，不依赖自然分钟边界，因此可以削平固定窗口的边界突刺。
	// KEYS[1]：令牌桶 key，例如 video_upload:rate:user:{user_id} 或 video_upload:rate:ip:{ip}。
	// ARGV[1]：桶容量 capacity，最多能积攒多少 token；容量越大，允许的短时 burst 越大。
	// ARGV[2]：补充速率 refill_rate，单位 token/秒，例如 120 次/分钟 = 2 token/秒。
	// ARGV[3]：当前时间 now_ms，单位毫秒，由 Go 传入，避免 Redis 版本差异影响 TIME 解析。
	// ARGV[4]：本次请求消耗 token 数，普通上传请求消耗 1。
	// ARGV[5]：Redis key 过期秒数，长时间无请求时自动清理桶状态。
	// 执行逻辑：
	// 1. 读取桶内剩余 tokens 和上次更新时间 updated_at。
	// 2. 按 elapsed_seconds * refill_rate 计算这段时间应补充多少 token，但不超过 capacity。
	// 3. 如果 tokens 足够支付本次请求，则扣减 token 并返回 1；否则不扣减并返回 0。
	// 4. 更新 tokens/updated_at，并设置过期时间。
	// 为什么用 Lua：读取、补充、扣减、写回必须原子执行；拆成多条 Redis 命令会被并发请求穿插，导致超发。
	TokenBucketRateLimitLuaScript = `
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])
local ttl_seconds = tonumber(ARGV[5])

local bucket = redis.call('HMGET', KEYS[1], 'tokens', 'updated_at')
local tokens = tonumber(bucket[1])
local updated_at = tonumber(bucket[2])

if tokens == nil then
  tokens = capacity
end
if updated_at == nil then
  updated_at = now_ms
end

local elapsed_seconds = math.max(0, now_ms - updated_at) / 1000
tokens = math.min(capacity, tokens + elapsed_seconds * refill_rate)

local allowed = 0
if tokens >= requested then
  tokens = tokens - requested
  allowed = 1
end

redis.call('HMSET', KEYS[1], 'tokens', tokens, 'updated_at', now_ms)
redis.call('EXPIRE', KEYS[1], ttl_seconds)

return allowed
`

	// ReleaseSubmitLockLuaScript 是安全释放锁的 Lua 脚本。
	// 先校验锁的 value 是否匹配，匹配才删除，防止误删其他请求持有的锁。
	// KEYS[1]：锁的 key。
	// ARGV[1]：锁的 value（UUID），用于校验是否为当前请求持有的锁。
	// 执行逻辑：
	// 1. 使用 GET 命令获取锁的当前 value。
	// 2. 与传入的 ARGV[1] 进行比较。
	// 3. 如果匹配，说明是当前请求持有的锁，执行 DEL 命令删除。
	// 4. 如果不匹配，说明锁已过期或被其他请求持有，返回 0。
	// 为什么用 Lua：校验和删除必须原子执行；拆成 GET + DEL 两条命令会被并发请求穿插，导致误删。
	ReleaseSubmitLockLuaScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end`
)

/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-22 14:26:12
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-22 14:26:16
 * @FilePath: /MLC_GO/internal/pkg/redis/hg_redis_script.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package PersistenceRedisPackage

const VideoInteractionOptimisticLuaScript = `
local stateKey = KEYS[1]
local countKey = KEYS[2]
local action = ARGV[1]
local active = ARGV[2]
local quantity = tonumber(ARGV[3]) or 0
local field = action

if action == 'share' then
  redis.call('HINCRBY', countKey, 'share', 1)
  return 1
end

if action == 'coin' then
  local current = tonumber(redis.call('HGET', stateKey, 'coin') or '0')
  if current + quantity > 2 then return 0 end
  redis.call('HINCRBY', stateKey, 'coin', quantity)
  redis.call('HINCRBY', countKey, 'coin', quantity)
  return 1
end

local current = redis.call('HGET', stateKey, field) or '0'
if current == active then return 0 end
redis.call('HSET', stateKey, field, active)
local delta = active == '1' and 1 or -1
local nextCount = redis.call('HINCRBY', countKey, field, delta)
if nextCount < 0 then redis.call('HSET', countKey, field, 0) end
return 1
`

// VideoDanmakuConsumeTicketLuaScript 原子读取并删除一次性 WebSocket 票据，阻止重放。
// KEYS[1] 是票据 key；成功返回绑定的 JSON，票据不存在或已消费时返回 nil。
const VideoDanmakuConsumeTicketLuaScript = `
local value = redis.call('GET', KEYS[1])
if not value then return nil end
redis.call('DEL', KEYS[1])
return value`

// VideoDanmakuRecentProjectLuaScript 原子完成 offset 去重、近期 Stream 写入和近似裁剪。
// KEYS[1] 是 Stream，KEYS[2] 是 offset Hash；ARGV[1] 是 topic:partition，ARGV[2] 是 offset，
// ARGV[3] 是最大条数，后续参数为 XADD field/value。两个 key 必须使用同一视频 hash tag。
const VideoDanmakuRecentProjectLuaScript = `
local lastOffset = redis.call('HGET', KEYS[2], ARGV[1])
if lastOffset and tonumber(ARGV[2]) <= tonumber(lastOffset) then
  return 0
end
redis.call('XADD', KEYS[1], 'MAXLEN', '~', ARGV[3], '*',
  'danmaku_id', ARGV[4], 'submission_id', ARGV[5], 'video_id', ARGV[6],
  'content', ARGV[7], 'progress_ms', ARGV[8], 'mode', ARGV[9],
  'color', ARGV[10], 'font_size', ARGV[11], 'created_at', ARGV[12])
redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])
return 1`

// InteractionReprojectCommitLuaScript atomically fences the lease owner, stores its checkpoint, and releases the lease.
// KEYS[1] is the lease and KEYS[2] is the checkpoint; ARGV[1] is the owner token and ARGV[2] is the encoded cursor.
const InteractionReprojectCommitLuaScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then
  return 0
end
redis.call('SET', KEYS[2], ARGV[2])
redis.call('DEL', KEYS[1])
return 1`

// InteractionReprojectReleaseLuaScript releases only the lease owned by the supplied token.
const InteractionReprojectReleaseLuaScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0`

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

	// VideoExternalCounterIncrementIfPresentLuaScript increments external only after the complete counter hash is initialized.
	// KEYS[1] is video_status_counter; ARGV[1] is the inserted external-content delta.
	// Checking all three fields prevents a crawler write from creating a partial hash that would be mistaken for an initialized counter.
	VideoExternalCounterIncrementIfPresentLuaScript = `
if redis.call('HEXISTS', KEYS[1], 'reviewing') == 0 or
   redis.call('HEXISTS', KEYS[1], 'published') == 0 or
   redis.call('HEXISTS', KEYS[1], 'external') == 0 then
    return 0
end
redis.call('HINCRBY', KEYS[1], 'external', ARGV[1])
return 1`

	// FeedPublishLuaScript 原子完成 Kafka offset 水位去重、ZSET 写入和容量裁剪。
	// KEYS[1]：分片 Feed ZSET；KEYS[2]：分片 offset 水位 Hash，两者必须使用同一 Redis Cluster hash tag。
	// ARGV[1]：submission_id；ARGV[2]：发布时间毫秒；ARGV[3]：最大成员数；ARGV[4]：topic:partition；ARGV[5]：offset。
	// 整体业务流程梳理
	// 1. 传入一条feed消息，携带submissionID、score、offset
	// 2. 根据submissionID哈希分到对应shard
	// 3. 执行Lua脚本（**原子执行，不会中间被其他命令打断**，Lua在redis单线程运行）
	//     1. 查历史offset水位
	//     2. 如果本条offset<=历史，直接返回0，什么都不改
	//     3. 否则ZADD把submissionID写入zset
	//     4. 如果zset总条数超过maxItems，删除分数最小的旧条目
	//     5. 更新hash水位为当前offset
	//     6. 返回1成功
	FeedPublishLuaScript = `
local lastOffset = redis.call('HGET', KEYS[2], ARGV[4])
if lastOffset and tonumber(ARGV[5]) <= tonumber(lastOffset) then
  return 0
end
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
local overflow = redis.call('ZCARD', KEYS[1]) - tonumber(ARGV[3])
if overflow > 0 then
  redis.call('ZREMRANGEBYRANK', KEYS[1], 0, overflow - 1)
end
redis.call('HSET', KEYS[2], ARGV[4], ARGV[5])
return 1`

	// FeedDeleteLuaScript 原子完成 Kafka offset 水位去重和 ZSET 删除。
	// ARGV[1]：submission_id；ARGV[2]：topic:partition；ARGV[3]：offset。
	FeedDeleteLuaScript = `
local lastOffset = redis.call('HGET', KEYS[2], ARGV[2])
if lastOffset and tonumber(ARGV[3]) <= tonumber(lastOffset) then
  return 0
end
redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('HSET', KEYS[2], ARGV[2], ARGV[3])
return 1`

	// VideoStatisticIncrementLuaScript 原子完成 Kafka offset 水位去重和分片 Hash 计数。
	// KEYS[1]：统计 Hash；KEYS[2]：offset 水位 Hash；ARGV[1]：事件名；ARGV[2]：topic:partition；ARGV[3]：offset。
	VideoStatisticIncrementLuaScript = `
local lastOffset = redis.call('HGET', KEYS[2], ARGV[2])
if lastOffset and tonumber(ARGV[3]) <= tonumber(lastOffset) then
  return 0
end
redis.call('HINCRBY', KEYS[1], ARGV[1], 1)
redis.call('HSET', KEYS[2], ARGV[2], ARGV[3])
return 1`
)

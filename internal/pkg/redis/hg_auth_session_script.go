package PersistenceRedisPackage

// HGAuthRegisterSessionLuaScript 原子登记新会话并淘汰本用户最久未续期的会话。
// KEYS: [1] 每用户 ZSET 索引，[2] 新会话 key，[3] 新 refresh 状态 key，[4] 用户删除隔离 key。
// ARGV: [1] sid，[2] refresh JTI，[3] TTL 秒，[4] 会话 key 前缀（含用户 ID），[5] refresh key 前缀（含用户 ID）。
// 返回 1 成功、0 账号隔离；淘汰时同时删除旧会话及已登记的 refresh 状态。
const HGAuthRegisterSessionLuaScript = `
if redis.call('EXISTS', KEYS[4]) ~= 0 then return 0 end
local now = redis.call('TIME')
local score = tonumber(now[1]) * 1000000 + tonumber(now[2])
redis.call('SET', KEYS[2], ARGV[2], 'EX', ARGV[3])
redis.call('SET', KEYS[3], '1', 'EX', ARGV[3])
redis.call('ZADD', KEYS[1], score, ARGV[1])
while redis.call('ZCARD', KEYS[1]) > 100 do
  local oldest = redis.call('ZPOPMIN', KEYS[1])[1]
  local key = ARGV[4] .. oldest
  local jti = redis.call('GET', key)
  redis.call('DEL', key)
  if jti and jti ~= '1' then redis.call('DEL', ARGV[5] .. jti) end
end
redis.call('EXPIRE', KEYS[1], ARGV[3])
return 1`

// HGAuthRefreshSessionLuaScript 原子校验未撤销的 refresh/会话并滑动续期，刷新不重建也不轮换令牌。
// KEYS: [1] 用户 ZSET 索引，[2] 会话 key，[3] refresh 状态 key，[4] 用户删除隔离 key。
// ARGV: [1] sid，[2] refresh JTI，[3] TTL 秒，[4] 会话 key 前缀，[5] refresh key 前缀。
// 返回 1 成功、0 已过期或撤销；旧版会话值 '1' 允许首次刷新登记索引。
const HGAuthRefreshSessionLuaScript = `
if redis.call('EXISTS', KEYS[4]) ~= 0 then return 0 end
local jti = redis.call('GET', KEYS[2])
if not jti or (jti ~= ARGV[2] and jti ~= '1') or redis.call('EXISTS', KEYS[3]) ~= 1 then return 0 end
if redis.call('TTL', KEYS[2]) <= 0 or redis.call('TTL', KEYS[3]) <= 0 then return 0 end
local now = redis.call('TIME')
local score = tonumber(now[1]) * 1000000 + tonumber(now[2])
redis.call('SET', KEYS[2], ARGV[2], 'EX', ARGV[3])
redis.call('EXPIRE', KEYS[3], ARGV[3])
redis.call('ZADD', KEYS[1], score, ARGV[1])
while redis.call('ZCARD', KEYS[1]) > 100 do
  local oldest = redis.call('ZPOPMIN', KEYS[1])[1]
  local key = ARGV[4] .. oldest
  local oldJTI = redis.call('GET', key)
  redis.call('DEL', key)
  if oldJTI and oldJTI ~= '1' then redis.call('DEL', ARGV[5] .. oldJTI) end
end
redis.call('EXPIRE', KEYS[1], ARGV[3])
if redis.call('EXISTS', KEYS[2]) ~= 1 then return 0 end
return 1`

// HGAuthRevokeSessionLuaScript 原子撤销登录会话并清理会话索引与对应 refresh 状态。
// KEYS: [1] 用户 ZSET 索引，[2] 会话 key；ARGV: [1] sid，[2] refresh key 前缀（含用户 ID）。
// 旧版会话值 '1' 没有记录 refresh JTI，但删除会话后旧 refresh 仍不能再续期。
const HGAuthRevokeSessionLuaScript = `
local jti = redis.call('GET', KEYS[2])
redis.call('DEL', KEYS[2])
redis.call('ZREM', KEYS[1], ARGV[1])
if jti and jti ~= '1' then redis.call('DEL', ARGV[2] .. jti) end
return 1`

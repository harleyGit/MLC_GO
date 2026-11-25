package PersistenceRedisPackage

// HGEmailSaveCodeLua 原子启用确认投递的注册邮件码，过期前最多允许五次猜测。
// KEYS: [1] 挑战 key；ARGV: [1] 绑定邮箱的验证码摘要，[2] 有效秒数。
const HGEmailSaveCodeLua = `
redis.call('HSET',KEYS[1],'digest',ARGV[1],'attempts',0)
redis.call('EXPIRE',KEYS[1],ARGV[2])
return 1`

// HGEmailVerifyCodeLua 在单键中消费注册邮件码，错误尝试最多五次，不延长 TTL。
// KEYS: [1] 挑战 key；ARGV: [1] 待比较摘要。
const HGEmailVerifyCodeLua = `
local digest = redis.call('HGET',KEYS[1],'digest')
if not digest then return 0 end
if digest == ARGV[1] then
  redis.call('DEL', KEYS[1])
  return 1
end
if redis.call('HINCRBY',KEYS[1],'attempts',1) >= 5 then redis.call('DEL', KEYS[1]) end
return 0`

// HGFixedWindowCountLua 原子维护固定窗口配额；满载不增加计数或延长窗口。
// KEYS: [1] 计数 key；ARGV: [1] 窗口秒数，[2] 窗口上限；返回 1 放行、0 限流。
const HGFixedWindowCountLua = `
local count = tonumber(redis.call('GET',KEYS[1]) or '0')
if count >= tonumber(ARGV[2]) then return 0 end
if redis.call('INCR',KEYS[1]) == 1 then redis.call('EXPIRE',KEYS[1],ARGV[1]) end
return 1`

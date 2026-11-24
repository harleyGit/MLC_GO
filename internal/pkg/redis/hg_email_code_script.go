package PersistenceRedisPackage

// HGEmailVerifyCodeLua 原子比较和一次性消费邮件码，错误尝试最多五次。
// KEYS: [1] 邮件码状态，[2] 同邮箱错误计数；ARGV: [1] 用户输入的六位码。
const HGEmailVerifyCodeLua = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  redis.call('DEL', KEYS[1], KEYS[2])
  return 1
end
local attempts = redis.call('INCR', KEYS[2])
if attempts == 1 then redis.call('EXPIRE', KEYS[2], 300) end
if attempts >= 5 then redis.call('DEL', KEYS[1]) end
return 0`

// HGEmailDiscardCodeLua 发信失败时只删除本次生成的码，不误删并发发送的新码。
// KEYS: [1] 邮件码状态；ARGV: [1] 本次码。
const HGEmailDiscardCodeLua = `
if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) end
return 0`

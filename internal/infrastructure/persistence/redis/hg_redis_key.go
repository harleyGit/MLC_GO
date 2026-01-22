/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-21 21:17:38
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-22 14:32:43
* @FilePath: /MLC_GO/internal/infrastructure/persistence/redis/hg_redis_key.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
*/
package PersistenceRedisPackage

/*
验证码：
auth:code:{phone}                → string，TTL 5min

验证码频控：
auth:code:limit:phone:{phone}    → int，TTL 1min
auth:code:limit:ip:{ip}          → int，TTL 1min

登录态：
auth:token:{userId}:{jti}        → string，TTL = access token TTL
auth:refresh:{userId}:{jti}      → string，TTL = refresh token TTL
*/

const (
	AuthCodePhoneLimitKey = "auth:code:limit:phone:"
	AuthCodeIPLimitKey    = "auth:code:limit:phone:"
	AuthTokenKey          = "auth:token:"
	AuthRefreshKey        = "auth:refresh:"
)

/* lua脚本 */
const (
	// 登录验证码和ip次数
	SmsLuaScript = `
	local current = redis.call("INCR", KEYS[1])
	if current == 1 then
		redis.call("EXPIRE", KEYS[1], ARGV[1])
	end

	return current
	`
)

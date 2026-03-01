/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-21 21:17:38
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-26 10:58:11
* @FilePath: /MLC_GO/internal/infrastructure/persistence/redis/hg_redis_key.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
*/
package PersistenceRedisPackage

import (
	"fmt"
	"strings"
)

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

type HGCacheKey string //TODO: https://www.qianwen.com/chat/2178247bfccf4511b6957cb4c7ca9227
// 为该类型定义字符串常量（这就是“自定义字符串枚举”）
const (
	StatusPending HGCacheKey = "pending"
	StatusActive  HGCacheKey = "active"
	StatusClosed  HGCacheKey = "closed"
)

const (
	AuthCodePhoneLimitKey   = "auth:code:limit:phone:"  // TODO：要改为注册发送的验证码Key
	AuthLoginVerifyCodekKey = "auth:login:verify:code:" // 登录验证码Key：手机、邮箱
	AuthCodeIPLimitKey      = "auth:code:limit:phone:"
	AuthTokenKey            = "auth:token:"
	AuthRefreshKey          = "auth:refresh:"

	UserListKey = "user:list:page:%d:size:%d" // user:list:{page}:{size} 获取注册用户列表的Key

	LoginCodeKey      = "login:code:"
	LoginMultiportKey = "token:" //token+多端登录控制key
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

// 可选：实现 String() 方法（其实可以直接用 string(s)）
func (key HGCacheKey) String() string {
	return string(key)
}
func GetRedisVerifyCodeKey(value string) string {
	// TODO: 这个需要改进下，因为该验证码用于注册，不是登录后面修改下
	return fmt.Sprintf("%s%s", AuthCodePhoneLimitKey, value)
}
func GetCacheKey(prefix string, value string) string {
	return fmt.Sprintf("%s%s", prefix, value)
}
func GetMultiportKey(uid int64, device string) string {
	key := fmt.Sprintf("%s%d:%s", LoginMultiportKey, uid, device)
	return key
}

// 自定义方法：接收任意数量的字符串参数，拼接成一个字符串
// 使用 Go 的变长参数（variadic parameters）: strs ...string
func (key HGCacheKey) WithSuffixes(strs ...string) string {
	base := key.String()
	if len(strs) == 0 {
		return base
	}
	// 将所有传入的字符串用 "-" 连接（你也可以用空格、逗号等）
	suffix := strings.Join(strs, "-")
	return base + "_" + suffix
}

// 可选：验证是否是合法值
func (key HGCacheKey) CacheKey() bool {
	switch key {
	case StatusPending, StatusActive, StatusClosed:
		return true
	default:
		return false
	}
}

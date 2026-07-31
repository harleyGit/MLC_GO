/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-21 21:17:38
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-06-08 15:06:31
* @FilePath: /MLC_GO/internal/pkg/redis/hg_redis_key.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
*/
package PersistenceRedisPackage

import (
	"fmt"
	"hash/fnv"
	"strings"
)

const (
	VideoInteractionStateKeyPrefix          = "video:interaction:state:"
	VideoInteractionCountKeyPrefix          = "video:interaction:count:"
	UserFollowStateKeyPrefix                = "user:follow:state:"
	UserFollowCountKeyPrefix                = "user:follow:count:"
	InteractionReprojectLeaseKeyPrefix      = "interaction:reproject:{control}:lease:"
	InteractionReprojectCheckpointKeyPrefix = "interaction:reproject:{control}:checkpoint:"
	CoinJobLeaseKey                         = "coin:jobs:{global}:lease"
)

func GetVideoInteractionStateKey(userID string, submissionID string) string {
	return fmt.Sprintf("%s{%s}:%s", VideoInteractionStateKeyPrefix, submissionID, userID)
}

func GetInteractionReprojectLeaseKey(stream string) string {
	return InteractionReprojectLeaseKeyPrefix + stream
}

func GetInteractionReprojectCheckpointKey(stream string) string {
	return InteractionReprojectCheckpointKeyPrefix + stream
}

func GetVideoInteractionCountKey(submissionID string) string {
	return fmt.Sprintf("%s{%s}", VideoInteractionCountKeyPrefix, submissionID)
}

func GetUserFollowStateKey(followerID string, followeeID string) string {
	return fmt.Sprintf("%s{%s}:%s", UserFollowStateKeyPrefix, followeeID, followerID)
}

func GetUserFollowCountKey(followeeID string) string {
	return fmt.Sprintf("%s{%s}", UserFollowCountKeyPrefix, followeeID)
}

// GetFeedShard 使用稳定哈希把同一 submission 固定到同一 Feed shard。
func GetFeedShard(submissionID string, shardCount int) int {
	if shardCount <= 1 {
		return 0
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(submissionID))
	return int(hasher.Sum32() % uint32(shardCount))
}

// GetFeedPublishedZSetKey 返回版本化分片 Feed ZSET；hash tag 将同 shard 的 Lua keys 固定到同一 slot。
func GetFeedPublishedZSetKey(generation string, shard int) string {
	return fmt.Sprintf("feed:%s:{feed-%04d}:published", generation, shard)
}

// GetFeedOffsetWatermarkKey 返回每个 Feed shard 的 Kafka partition offset 水位 Hash。
func GetFeedOffsetWatermarkKey(generation string, shard int) string {
	return fmt.Sprintf("feed:%s:{feed-%04d}:offsets", generation, shard)
}

// GetStatisticShard 将 Kafka partition 稳定分散到统计 counter shards。
func GetStatisticShard(partition int32, shardCount int) int {
	if shardCount <= 1 {
		return 0
	}
	if partition < 0 {
		partition = -partition
	}
	return int(partition) % shardCount
}

// GetVideoEventCounterKey 返回版本化统计分片 Hash。
func GetVideoEventCounterKey(generation string, shard int) string {
	return fmt.Sprintf("statistic:%s:{stat-%04d}:events", generation, shard)
}

// GetVideoStatisticOffsetWatermarkKey 返回统计分片的 Kafka offset 水位 Hash。
func GetVideoStatisticOffsetWatermarkKey(generation string, shard int) string {
	return fmt.Sprintf("statistic:%s:{stat-%04d}:offsets", generation, shard)
}

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
	AuthCodePhoneLimitKey    = "auth:code:limit:phone:"    // TODO：要改为注册发送的验证码Key
	AuthLoginVerifyCodekKey  = "auth:login:verify:code:"   // 登录验证码Key：手机、邮箱
	AuthResetPasswordCodeKey = "auth:reset:password:code:" // 忘记密码验证码Key：手机、邮箱
	AuthCodeIPLimitKey       = "auth:code:limit:phone:"
	AuthTokenKey             = "auth:token:"
	AuthRefreshKey           = "auth:refresh:"

	UserListKey        = "user:list:cursor:%d:size:%d" // user:list:{cursor}:{size} 获取注册用户列表的Key
	UserListPatternKey = "user:list:cursor:*:size:*"   // 删除用户列表分页缓存时使用
	UserListTotalKey   = "user:list:total"

	LoginCodeKey      = "login:code:"
	LoginMultiportKey = "token:" //token+多端登录控制key

	VideoUploadSessionKeyPrefix  = "video_upload:session:"
	VideoUploadUserRateKeyPrefix = "video_upload:rate:user:"
	VideoUploadIPRateKeyPrefix   = "video_upload:rate:ip:"
	// 提交任务锁，防止用户重复提交任务【视频上传草稿】
	VideoUploadSubmitLockKeyPrefix   = "video_upload:submit_lock:"
	VideoUploadSubmitResultKeyPrefix = "video_upload:submit_result:"
	VideoUploadListPageKeyPrefix     = "video_upload:list:cursor:"
	VideoUploadListPagePatternKey    = "video_upload:list:cursor:*"
	// VideoStatusCounterKey 用 Redis Hash 按稿件状态维护视频列表计数，避免高并发列表总数查询打到 MySQL COUNT(*)。
	VideoStatusCounterKey = "video_status_counter"
	// FeedPublishedZSetKey 保存已发布视频，score 为发布时间毫秒，member 为 submission_id。
	// {global} 让 ZSET 与幂等 key 在 Redis Cluster 中可通过同一 hash tag 落到同一 slot，满足 Lua 原子执行约束。
	FeedPublishedZSetKey     = "feed:{global}:published"
	FeedIdempotencyKeyPrefix = "feed:{global}:idempotency:"
	// VideoEventCounterKey 按事件名维护预聚合计数，避免实时扫描亿级业务表。
	VideoEventCounterKey               = "statistic:{video}:events"
	VideoStatisticIdempotencyKeyPrefix = "statistic:{video}:idempotency:"
	// OpsBilibiliActiveTagListKey 缓存动画页启用标签，写操作提交后直接删除该固定 key。
	OpsBilibiliActiveTagListKey = "ops:bilibili:douga_tags:active"
)

// GetFeedIdempotencyKey 生成 Feed 投影事件幂等 key。
func GetFeedIdempotencyKey(eventID string) string {
	return FeedIdempotencyKeyPrefix + eventID
}

// GetVideoStatisticIdempotencyKey 生成视频统计事件幂等 key。
func GetVideoStatisticIdempotencyKey(eventID string) string {
	return VideoStatisticIdempotencyKeyPrefix + eventID
}

/* lua脚本 */
const (
	// 登录验证码和ip次数， TODO：有临界突刺问题，用 TokenBucketRateLimitLuaScript 这个，这个解决了
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

// GetRedisEmailVerifyCodeKey 生成邮箱验证码的 Redis key。
// 使用独立前缀隔离邮箱验证码，避免与手机验证码冲突。
func GetRedisEmailVerifyCodeKey(email string) string {
	return fmt.Sprintf("auth:email:code:%s", email)
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

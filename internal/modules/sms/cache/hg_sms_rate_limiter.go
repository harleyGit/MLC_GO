/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 21:29:04
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-22 14:43:35
 * @FilePath: /MLC_GO/internal/modules/sms/cache/hg_sms_rate_limiter.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGSMSCachePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type HGSMSRateLimiter struct {
	rdb *redis.Client
}

func NewSMSRateLimiter(rdb *redis.Client) *HGSMSRateLimiter {
	return &HGSMSRateLimiter{rdb: rdb}
}

// TODO: 后面这个脚本放在基础的redis里面
func (r *HGSMSRateLimiter) CheckLoginCountWithLua(key string, 
	limit int64, window time.Duration) (count int64, exceeded bool, err error) {

		result, err := r.rdb.Eval(context.Background(), PersistenceRedisPackage.SmsLuaScript, []string{key}, limit, int(window.Seconds())).Result()
		if err != nil {
			return 0, false, err
		}
		count = result.(int64)
		return count, count > limit, nil
}

func (r *HGSMSRateLimiter) Check(ctx context.Context, phone, ip string) error {
	phoneKey := fmt.Sprintf("%s%s", PersistenceRedisPackage.AuthCodePhoneLimitKey, phone)
	ipKey := fmt.Sprintf("%s%s", PersistenceRedisPackage.AuthCodeIPLimitKey, ip)

	/* // 存在问题： 中间可能被其他请求打断；极端情况：第一次请求执行了 INCR，但还没 EXPIE 就当机了，导致key永久存在
		// 将key中的值+1，若key不存在，则先初始化0，再执行增加操作，返回一个执行结果
		p := r.rdb.Incr(ctx, phoneKey) // phoneKey 的值 +1
		i := r.rdb.Incr(ctx, ipKey)

		// 设置key 的过期时间，到达后key会被自动删除
		r.rdb.Expire(ctx, phoneKey, time.Minute)
		r.rdb.Expire(ctx, ipKey, time.Minute)

		if p.Val() > 5 { // 获取phoneKey的当前值，是否大于5
			return errors.New("手机号发送过于频繁")
		}

		if i.Val() > 20 {
			return errors.New("IP 请求过多")
		}
	*/
	// 使用lua脚本的原子特性，解决上述问题
	_, phoneExceed, _ := r.CheckLoginCountWithLua(phoneKey, 5, time.Minute)
	if phoneExceed { // 获取phoneKey的当前值，是否大于5
		return errors.New("手机号发送过于频繁")
	}
	_, ipExceed, _ := r.CheckLoginCountWithLua(ipKey, 10, time.Minute)

	if ipExceed {
		return errors.New("IP 请求过多")
	}
	
	return nil
}

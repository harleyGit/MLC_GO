/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-13 15:42:07
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-13 15:49:04
 * @FilePath: /MLC_GO/TestNotes/RedisExample/RedisPracticeExample.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
// 启动redis服务: redis-server
package redisexample

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"

	"github.com/gomodule/redigo/redis"
)

// .int类型数据插入与查询, 控制台出现：9990，就代表成功了
func RedisExamplePractice() {
	c, err := redis.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		logging.ErrInfo("连接 redis 失败 err:", err)
		return
	}

	defer c.Close()
	_, err = c.Do("Set", "abc", 9990)
	if err != nil {
		logging.ErrInfo("redis 操作失败")
		return
	}

	r, err := redis.Int(c.Do("Get", "abc"))
	if err != nil {
		logging.ErrInfo("redis get abc 操作失败")
		return
	}

	logging.DebugInfo(r)
}


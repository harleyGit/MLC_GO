/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-11 12:33:42
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-11 12:47:51
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Server/redis.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package server

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

// 定义一个全局的pool
var pool *redis.Pool

func initPool(address string, maxIdle, maxActive int, idleTimeout time.Duration) {

	pool = &redis.Pool{
		MaxIdle: maxIdle,//最大空闲连接数
		MaxActive: maxActive,//表示和数据库的最大连接数； 0 表示没有限制
		IdleTimeout: idleTimeout,//最大空闲时间
		Dial: func() (redis.Conn, error) {//初始化连接代码，链接哪个
			return redis.Dial("tcp", address)
		},
	}
}
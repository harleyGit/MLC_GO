/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-13 16:34:32
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-13 17:09:08
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/pkg/gredis/redis.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gredis

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/setting"
	"encoding/json"
	"time"

	"github.com/gomodule/redigo/redis"
)

var RedisConn *redis.Pool

func Setup() error {
	// 初始化一个 Redis 连接池，用于管理与 Redis 服务器的连接，避免每次操作都建立和销毁连接，提高性能。
	RedisConn = &redis.Pool{
		MaxIdle: setting.RedisSetting.MaxIdle, // 指定连接池中允许的最大空闲连接数。超过这个数的空闲连接会被关闭，以节约资源。
		MaxActive: setting.RedisSetting.MaxActive, // 设置连接池中允许的最大活动连接数（即同时使用的连接数）。当达到这个限制时，新的连接请求将会被阻塞，直到有连接释放回池中。
		IdleTimeout: setting.RedisSetting.IdleTimeout, // 定义空闲连接的超时时间。超过此时间未被使用的连接会被关闭，防止资源浪费。
		Dial: func() (redis.Conn, error) { // 用于建立一个新的 Redis 连接。
			c, err := redis.Dial("tcp", setting.RedisSetting.Host) // 通过 TCP 协议连接到 Redis
			if err != nil {
				return nil, err
			}
			if setting.RedisSetting.Password != "" {
				if _, err := c.Do("AUTH", setting.RedisSetting.Password); err != nil { // 发送 AUTH 命令进行验证。
					c.Close()
					return nil, err
				}
			}
			return c, err
		},

		TestOnBorrow: func(c redis.Conn, lastUsed time.Time) error { // 在连接从池中借出前进行健康检查，确保连接依然有效。
			_, err := c.Do("PING") // 通过 c.Do("PING") 检查连接是否活跃
			return err
		},
	}

	return nil
}


func Set(key string, data interface{}, time int) error {
	conn := RedisConn.Get() //  从连接池中获取一个 Redis 连接对象。这样可以避免每次操作都创建新的连接，提高性能。
	defer conn.Close() // 确保在函数结束时，无论函数是正常退出还是遇到错误，都能自动将连接归还给连接池，释放资源。

	// 将 Go 语言中的数据结构（data）转换成 JSON 格式的字节切片。这通常用于在 Redis 中存储结构化数据。
	value, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// 向 Redis 发送 SET 命令，将键 key 对应的值设为刚刚转换成 JSON 格式的 value。
	_, err = conn.Do("SET", key, value)
	if err != nil {
		return err
	}

	// 向 Redis 发送 EXPIRE 命令，为键 key 设置过期时间。这里的 time 通常表示秒数，即多少秒后该键会被自动删除。
	_, err = conn.Do("EXPIRE", key, time) 
	if err != nil {
		return err
	}

	return nil
}

// 判断 Redis 中是否存在指定的键。
func Exists(key string) bool {
	conn := RedisConn.Get()
	defer conn.Close()

	// 将命令返回转为布尔值
	// 使用 conn.Do("EXISTS", key) 执行 Redis 的 EXISTS 命令，该命令会检查指定的 key 是否存在。
	// 调用 redis.Bool() 将返回结果转换为布尔类型。如果命令执行成功，返回 true 表示键存在，否则为 false。
	exists, err := redis.Bool(conn.Do("EXISTS", key))
	if err != nil {
		return false
	}

	return exists
}

// 从 Redis 中获取指定 key 对应的值，并以字节切片形式返回。
func Get(key string) ([]byte, error) {
	conn := RedisConn.Get()
	defer conn.Close()

	// 将命令返回转为 Bytes
	// 使用 conn.Do("GET", key) 执行 Redis 的 GET 命令。
	// 调用 redis.Bytes() 将返回的结果转换为字节切片（[]byte）。
	reply, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		return nil, err
	}

	return reply, nil
}

func Delete(key string) (bool, error) {
	conn := RedisConn.Get()
	defer conn.Close()

	// 通过 conn.Do("DEL", key) 执行 Redis 的 DEL 命令，用于删除指定的键。
	// 使用 redis.Bool() 将返回的结果转换为布尔类型。
	// 注意：在 Redis 中，DEL 命令返回的是删除的键的数量，转换为布尔值后，通常表示是否删除了至少一个键。
	return redis.Bool(conn.Do("DEL", key))
}

// 模糊删除：查找所有键名中包含指定字符串 key 的键，并依次删除。
func LikeDeletes(key string) error {
	conn := RedisConn.Get()
	defer conn.Close()

	// 将命令返回转为 []string  
	// 使用 conn.Do("KEYS", "*"+key+"*") 执行 Redis 的 KEYS 命令，查找所有键名包含给定字符串 key 的键。
	// 调用 redis.Strings() 将结果转换为字符串切片。
	keys, err := redis.Strings(conn.Do("KEYS", "*"+key+"*"))
	if err != nil {
		return err
	}

	for _, key := range keys {
		_, err = Delete(key)
		if err != nil {
			return err
		}
	}

	return nil
}



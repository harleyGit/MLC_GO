/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-13 15:42:07
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 09:52:59
 * @FilePath: /MLC_GO/TestNotes/RedisExample/RedisPracticeExample.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
// 启动redis服务: redis-server
package redisexample

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"

	redis_protocol_example "MLC_GO/TestNotes/RedisExample/RedisProtocolExample"
	"MLC_GO/TestNotes/RedisExample/RedisProtocolExample/protocol"
	"log"
	"os"

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

/*** func RedisProtocolExmPractice() 方法的测试 需要执行下面的方法步骤:
*使用:
*	Status Reply
		$ go run main.go SET test01 value01
		2018/06/06 21:29:07 Reply: OK
		2018/06/06 21:29:07 Command: +OK
*	
*	Error Reply
		$ go run main.go error
		2018/06/06 22:20:39 Reply: ERR unknown command 'error'
		2018/06/06 22:20:39 Command: -ERR unknown command 'error'

*	Integer Reply
		$ go run main.go EXPIRE test01 3600
		2018/06/06 22:18:00 Reply: 1
		2018/06/06 22:18:00 Command: :1

*	Multi Bulk Reply
		$ go run main.go LPUSH test-multi 01
		2018/06/06 22:23:50 Reply: 1
		2018/06/06 22:23:50 Command: :1

		$ go run main.go LPUSH test-multi 02
		2018/06/06 22:23:54 Reply: 2
		2018/06/06 22:23:54 Command: :2

		$ go run main.go LPUSH test-multi 03
		2018/06/06 22:23:57 Reply: 3
		2018/06/06 22:23:57 Command: :3

		$ go run main.go LRANGE test-multi 0 10
		2018/06/06 22:24:10 Reply: [03 02 01]
		2018/06/06 22:24:10 Command: *3
		$2
		03
		$2
		02
		$2
		01
*/
func RedisProtocolExmPractice() {
	args := os.Args[1:]
	if len(args) <= 0 {
		log.Fatalf("Os.Args <= 0")
	}

	reqCommand := protocol.GetRequest(args)
	redisConn, err := redis_protocol_example.Conn(redis_protocol_example.Network, redis_protocol_example.Address)
	if err != nil {
		log.Fatalf("Conn err: %v", err)
	}
	defer redisConn.Close()

	_, err = redisConn.Write(reqCommand)
	if err != nil {
		log.Fatalf("Conn Write err: %v", err)
	}

	command := make([]byte, 1024)
	n, err := redisConn.Read(command)
	if err != nil {
		log.Fatalf("Conn Read err: %v", err)
	}

	reply, err := protocol.GetReply(command[:n])
	if err != nil {
		log.Fatalf("protocol.GetReply err: %v", err)
	}

	log.Printf("Reply: %v", reply)
	log.Printf("Command: %v", string(command[:n]))
}





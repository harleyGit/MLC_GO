/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-13 20:05:45
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-13 20:09:05
 * @FilePath: /MLC_GO/TestNotes/RedisExample/RedisProtocolExample/RedisProtocolExm.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package redis_protocol_example

import (
	
	"net"
)

const (
	Address = "127.0.0.1:6379"
	Network = "tcp"
)

func Conn(network, address string) (net.Conn, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}

	return conn, nil
}


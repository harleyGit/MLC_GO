/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-23 10:54:26
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-23 11:33:44
 * @FilePath: /MLC_GO/TestNotes/TestNetProgram/TestUDPSocket/TestUDPServerSocket/test_udp_server_socket.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
 /// udp服务端接收数据
package main

import (
	"fmt"
	"net"
)

func main() {
	// udp服务端接收数据
	testUDPServerSocketV0()
}

func testUDPServerSocketV0() {
	// 使用
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP: net.IPv4(127,0,0,1),
		Port: 3000,
		Zone: "",
	})

	if err != nil {
		fmt.Println("监听失败！发送错误：", err)
		return
	}
	fmt.Println("服务端开启！ 等待客户端的连接请求.......")

	for{
		var data[1024]byte

		// 接收数据
		count, addr, err := conn.ReadFromUDP(data[:])
		if err != nil {
			fmt.Println("接收数据失败！发送错误：", err)
			continue
		}
		fmt.Printf("已成功接收数据： %s\n", data[0: count])

		
		// 发送数据
		_, errs := conn.WriteToUDP([]byte("👋你好！ 客户端！🍏"), addr)
		if errs != nil {
			fmt.Println("发送数据失败！ 发生错误❌：", errs)
			continue
		}
	}
}

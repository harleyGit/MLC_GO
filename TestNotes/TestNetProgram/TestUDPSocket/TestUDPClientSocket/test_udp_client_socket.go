/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-23 10:55:12
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-23 11:28:56
 * @FilePath: /MLC_GO/TestNotes/TestNetProgram/TestUDPSocket/TestUDPClientSocket/test_udp_client_socket.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
/// udp客户端接收数据
package main

import (
	"fmt"
	"net"
)

func main() {
	// udp客户端接收数据
	testUDPClientSocket()
}

func testUDPClientSocket() {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP: net.IPv4(127, 0, 0, 1),
		Port: 3000,
		Zone: "",
	})

	if err != nil {
		fmt.Println("连接失败！发生错误❌：", err)
		return
	}
	fmt.Println("客户端向服务器端发送连接请求......")
	defer conn.Close()

	// 发送数据
	sendData := []byte("👋你好，服务器🧧端！！")
	_, errs := conn.Write(sendData)
	if errs != nil {
		fmt.Println("发送数据失败！发生错误：", errs)
		return
	}

	// 接收数据
	data := make([]byte, 4096)
	_,_, errors := conn.ReadFromUDP(data)
	if errors != nil {
		fmt.Println("接收数据失败！ 发生错误：", errors)
		return
	}
	fmt.Printf("已成功接收数据： %s\n", string(data))
}
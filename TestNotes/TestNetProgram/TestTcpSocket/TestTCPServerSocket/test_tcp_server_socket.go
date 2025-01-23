/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-22 16:52:30
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-23 10:38:23
 * @FilePath: /MLC_GO/TestNotes/TestNetProgram/TestNetSocket/test_socketV0.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
 /// 服务端建立连接
package main

import (
	"fmt"
	"net"
)

func main() {
	// 创建TCP连接
	// testServerSocketListenV1()

	// TCP连接并发送数据
	testTCPServerSocketListenV2()
}

// TCP服务端连接、发送数据
func testTCPServerSocketListenV2(){
	// 使用 net.Listen() 函数监听连接的地址与端口
	listener, err := net.Listen("tcp", "127.0.0.1:3000")
	if err != nil {
		fmt.Printf("监听失败！发送错误：%v\n", err)
		return
	}
	fmt.Println("服务端已开启！等待客户端的连接请求.......")
	for{
		// 响应由TCP客户端发送的连接请求
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("响应失败！ 发生错误： %v\n", err)
			continue
		}
		// 对每个新连接创建的协程收发数据
		go process(conn)
	}

}
func process(conn net.Conn){
	defer conn.Close()

	for{
		var buf[128]byte
		// 接收数据
		n, err := conn.Read(buf[:])
		if err != nil {
			fmt.Printf("接收数据失败！ 发生错误：%v\n", err)
			break
		}
		fmt.Printf("已成功接收数据：%v\n", string(buf[:n]))
		// 发送数据
		if _, err = conn.Write([]byte("服务端消息！")); err != nil {
			fmt.Printf("发生数据失败！ 发生错误：%v\n", err)
			break
		}
	}
}

// TCP服务端服务- 创建TCP连接
func testServerSocketListenV1(){
	// 使用 net.Listen() 函数监听连接的地址与端口
	listener, err := net.Listen("tcp", "127.0.0.1:3000")
	if err != nil {
		fmt.Printf("监听失败！发送错误：%v\n", err)
		return
	}
	fmt.Println("服务端已开启！等待客户端的连接请求.......")
	// 响应由TCP客户端发送的连接请求
	conn, err := listener.Accept()
	if err != nil {
		fmt.Printf("响应失效！ 发生错误: %v\n", err)
	}
	fmt.Println("服务端已连接客户端！")
	defer conn.Close()
}


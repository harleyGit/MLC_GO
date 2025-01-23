/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-22 21:54:28
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-23 10:14:02
 * @FilePath: /MLC_GO/TestNotes/TestNetProgram/TestTCPClientSocket/test_tcp_client_socket.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
/// 客户端建立连接
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)


func main() {
	// 客户端连接
	//testClientConnect()

	// 客户端发送数据
	testTCPClientConnectV2()
}

// 客户端发送数据
func testTCPClientConnectV2() {
	conn, err := net.Dial("tcp", "127.0.0.1:3000")
	if err != nil {
		fmt.Printf("连接失败！发生错误：%v\n", err.Error())
		return
	}

	fmt.Println("客户端向服务器端发送连接请求......")
	defer conn.Close()
	inputReader := bufio.NewReader(os.Stdin)
	for{
		input, err  := inputReader.ReadString('\n')
		if err != nil {
			fmt.Printf("无法读取在控制台上输入的数据！发生错误： %v\n", err)
			break
		}
		trimmedInput := strings.TrimSpace(input)
		if trimmedInput == "Q" {
			break
		}
		// 发送数据
		if _,err := conn.Write([]byte(trimmedInput)); err != nil{
			fmt.Printf("发送数据失败！发生错误：%v\n", err)
			break
		}

		// 接收数据
		var recvData = make([]byte, 1024)
		if _,err := conn.Read(recvData); err != nil {
			fmt.Printf("接收数据失败！ 发生错误：%v\n", err)
			break
		}
		fmt.Printf("已成功接收数据： %v\n", string(recvData))
	}
}

func testClientConnect() {
	conn, err := net.Dial("tcp", "127.0.0.1:3000")	// 建立与服务端的连接
	if err != nil {
		fmt.Printf("连接失败！ 发生错误： %v\n", err.Error())
		return
	}

	fmt.Println("客户端向服务端发送连接请求.....")
	defer conn.Close()
}
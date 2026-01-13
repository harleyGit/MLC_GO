/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-09 21:27:05
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:04:42
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Server/ServerMainPractice.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 * 尚硅谷TCP服务资料Code： https://gitee.com/gtxy27/go/blob/main/chatroom/server/main/redis.go
 */
package server

import (
	"MLC_GO/TestNotes/SocketPractice/Server/model"
	"MLC_GO/internal/pkg/logHG"

	// "fmt"
	"net"
	"time"
	// "github.com/sourcegraph/conc/pool"
)

func init() {

	// 当服务器启动时，我们就去初始化我们的redis的连接池
	initPool("localhost: 6379", 16, 0, 30*time.Second)
	initUserDao()
}

// 服务端的路口函数
func ServerPracticeMain() {

	logHG.DebugInfo("服务器 【新的结构】在 8889 端口监听..... ")
	listen, err := net.Listen("tcp", "0.0.0.0: 8889")
	defer listen.Close()
	if err != nil {
		logHG.DebugInfo("net.listen err = ", err)
		return
	}

	// 一旦监听成功，就等待客户端来链接服务器
	for {
		logHG.DebugInfo("等待客户端来链接服务器.....")
		conn, err := listen.Accept()
		if err != nil {
			logHG.DebugInfo("listen.Accept err = ", err)
		}

		// 一旦链接成功，则启动一个携程和客户端保持通讯
		go process(conn)
	}
}

// 处理和客户端的通讯
func process(conn net.Conn) {

	// 这里需要延时关闭conn
	defer conn.Close()

	// 创建一个调用总控
	processor := &Processor{
		Conn: conn,
	}
	err := processor.process2()
	if err != nil {
		logHG.ErrInfo("客户端和服务器通讯携程 err:", err)
		return
	}
}

// 完成UserDao的初始化任务
func initUserDao() {
	// 这里的pool本身就是一个全局变量
	// 这里需要注意一个初始化顺序问题
	// initPool,在 initUserDao
	model.MyUserDao = model.NewUserDao(pool)
}

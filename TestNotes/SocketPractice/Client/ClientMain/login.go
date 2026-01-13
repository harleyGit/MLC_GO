/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-21 15:24:19
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:02:11
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Client/ClientMain/login.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package clientmain

import (
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/internal/pkg/logHG"
	"encoding/binary"
	"encoding/json"
	"net"
)

// 登录
func login(userId int, userPwd string) (err error) {

	// 1.连接到服务器
	conn, err := net.Dial("tcp", "localhost:8889")
	if err != nil {
		logHG.ErrInfo("客户端 net.dial err =", err)
		return
	}
	//延时关闭
	defer conn.Close()

	// 2.准备通过conn发送消息给服务
	var mes message.Message
	mes.Type = message.LoginMesType
	// 3.创建一个LoginMes 结构体 
	var loginMes message.LoginMes
	loginMes.UserId = userId
	loginMes.UserPWD = userPwd
	
	// 4.将loginMes序列化 
	data, err := json.Marshal(loginMes)
	if err != nil {
		logHG.ErrInfo("json.Marshal err=", err)
		return
	}
	// 5.把data赋给mes.data字段
	mes.Data = string(data)

	// 6.将将mes进行序列化
	data, err = json.Marshal(mes)
	if err != nil {
		logHG.ErrInfo("json.Marshal err =", err)
		return
	}
	
	// 7. 到这个时候data就是我们要发送的消息 
	// 7.1 先把data的长度发送给服务器 
	// 先获取到data的长度 -> 转成一个表示长度的 byte 切片 
	var pkgLen uint32
	pkgLen = uint32(len(data))
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[0:4], pkgLen)
	// 发送长度
	n, err := conn.Write(buf[:4])
	if n != 4  || err != nil {
		logHG.ErrInfo("conn.Write(bytes) fail: ", err)
		return
	}

	// 发送消息本身
	_, err = conn.Write(data)
	if err != nil {
		logHG.ErrInfo("conn.Write(data) fail: ", err)
		return
	}

	// 休眠20
	// 这里还需要处理服务器端返回的消息
	mes, err = readPkg(conn) // mes就是
	if err != nil {
		logHG.ErrInfo("readPkg(conn) err: ", err)
		return
	}

	// 将mes的Data部分反序列化成LoginResMes
	var loginResMes message.LoginResMes
	err = json.Unmarshal([]byte(mes.Data), &loginResMes)
	if loginResMes.Code == 200 {
		logHG.DebugInfo("登录成功")
	}else if loginResMes.Code == 500 {
		logHG.ErrInfo(loginResMes.Error)
	}

	return
}

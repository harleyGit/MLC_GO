package process

import (
	"MLC_GO/TestNotes/SocketPractice/Server/utils"
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/pkg/logHG"
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
)

type UserProcess struct {
}

func (this *UserProcess) Register(userId int, userPwd string, userName string) (err error) {

	// 1. 链接到服务器
	conn, err := net.Dial("tcp", "localhost:8889")
	if err != nil {
		logHG.ErrInfo("net.Dial err = ", err)
		return
	}

	//延时关闭
	defer conn.Close()

	// 2.准备通过conn发送消息给服务
	var mes message.Message
	mes.Type = message.RegisterMesType
	// 3.创建一个LoginMes结构体
	var registerMes message.RegisterMes
	registerMes.User.UserId = userId
	registerMes.User.UserPwd = userPwd
	registerMes.User.UserName = userName

	// 4.将registerMes序列化
	data, err := json.Marshal(registerMes)
	if err != nil {
		logHG.ErrInfo("json.Marshal err = ", err)
		return
	}

	// 5.把data赋值给mes.Data字段
	mes.Data = string(data)

	// 6.将 mes进行序列化
	data, err = json.Marshal(mes)
	if err != nil {
		logHG.ErrInfo("json.Marshal err =", err)
		return
	}

	//创建一个Transfer实例
	tf := &utils.Transfer{
		Conn: conn,
	}

	// 发送data给服务器
	err = tf.WritePkg(data)
	if err != nil {
		logHG.ErrInfo("注册发送信息错误 err：", err)
	}
	mes, err = tf.ReadPkg() //mes 就是 RegisterResMes

	if err != nil {
		logHG.ErrInfo("readPkg(conn) err = ", err)
		return
	}

	// 将mes的Data部分反序列化成RegisterResMes
	var registerResMes message.RegisterResMes
	err = json.Unmarshal([]byte(mes.Data), &registerResMes)
	if registerResMes.Code == 200 {
		logHG.DebugInfo("注册成功，可以重新登录了.....")
		os.Exit(0)
	} else {
		logHG.ErrInfo(registerResMes.Error)
		os.Exit(0)
	}
	return
}

// 给关联一个用户登录的方法
// 写一个函数，完成登录
func (this *UserProcess) Login(userId int, userPwd string) (err error) {

	// 1.链接到服务器
	conn, err := net.Dial("tcp", "localhost:8889")
	if err != nil {
		logHG.ErrInfo("net.Dial err =", err)
		return
	}

	// 延时关闭
	defer conn.Close()

	// 2.准备通过conn发送消息给服务
	var mes message.Message
	mes.Type = message.LoginMesType
	// 3.创建一个LoginMes 结构体
	var loginMes message.LoginMes
	loginMes.UserId = userId
	loginMes.UserPWD = userPwd

	// 4.将loginMes 序列化
	data, err := json.Marshal(loginMes)
	if err != nil {
		logHG.ErrInfo("json.Marshal err = ", err)
		return
	}

	// 5. 把 data赋值给mes.Data字段
	mes.Data = string(data)

	// 6.将mes进行序列化
	data, err = json.Marshal(mes)
	if err != nil {
		logHG.ErrInfo("json.Marshal err = ", err)
		return
	}

	// 7. 到这个时候 data就是我们要发送的消息
	// 7.1 先把 data的长度发送给服务器
	// 先获取到data的长度 -> 转成一个表示长度的byte切片
	var pkgLen uint32
	pkgLen = uint32(len(data))
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[0:4], pkgLen)
	// 发送长度
	n, err := conn.Write(buf[:4])
	if n != 4 || err != nil {
		logHG.ErrInfo("conn.Write(bytes) fail", err)
		return
	}
	logHG.DebugInfo("客户端，发送消息的长度=%d, 内容=%s", len(data), string(data))

	// 发送消息本身
	_, err = conn.Write(data)
	if err != nil {
		logHG.ErrInfo("conn.Write(data) fail", err)
		return
	}

	// 创建一个Transfer实例
	tf := &utils.Transfer{
		Conn: conn,
	}
	mes, err = tf.ReadPkg() // mes就是
	if err != nil {
		logHG.ErrInfo("readPkg(conn) err=", err)
		return
	}

	// 将mes的Data部分反序列化成LoginResMes
	var loginResMes message.LoginResMes
	err = json.Unmarshal([]byte(mes.Data), &loginResMes)
	if loginResMes.Code == 200 {
		// 初始化CurUser
		CurUser.Conn = conn
		CurUser.UserId = userId
		CurUser.UserStatus = message.UserOnlie

		// 可以显示当前在线用户列表，遍历loginResMes.UserId
		logHG.DebugInfo("当前在线用户列表如下：")
		for _, v := range loginResMes.UserId {
			// 如果我们要求不显示自己在线，下面我们增加一个代码
			if v == userId {
				continue
			}
			logHG.DebugInfo("用户ID：\t", v)
			// 完成客户端的onlineUsers完成初始化
			user := &message.User{
				UserId:     v,
				UserStatus: message.UserOffline,
			}
			onlineUsers[v] = user
		}
		logHG.DebugInfo("\n\n")

		// 这里我们还需要在客户端启动一个携程
		// 该携程保持和服务器的通讯，如果服务器又数九推送给客户端
		// 则接收并显示在客户端的终端
		go serverProcessMes(conn)

		// 1. 显示我们的登录成功菜单【循环】..
		for {
			ShowMenu()
		}
	} else {
		logHG.DebugInfo(loginResMes.Error)
	}
	return
}

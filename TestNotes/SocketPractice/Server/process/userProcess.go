/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-13 16:20:37
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:05:54
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Server/process/userProcess.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package process2

import (
	"MLC_GO/TestNotes/SocketPractice/Server/model"
	"MLC_GO/TestNotes/SocketPractice/Server/utils"
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/internal/pkg/logHG"
	"encoding/json"
	"net"
)

type UserProcess struct {
	// 字段
	Conn net.Conn
	// 表示Conn是哪个用户
	UserId int
}

/*
	通知所有在线的用户的方法

userId 要通知其他的在线用户，我要上线了
*/
func (this *UserProcess) NotifyOthersOnlineUser(userId int) {

	// 遍历 onlineUsers， 然后一个一个的发送 NotifyUserStatusMes
	for id, up := range userMgr.onlineUsers {
		// 过滤到自己
		if id == userId {
			continue
		}
		// 开始通知【单独的写一个方法】
		up.NotifyMeOnline(userId)
	}
}

/* 通知我在线人数 */
func (this *UserProcess) NotifyMeOnline(userId int) {

	// 组装我们的NotifyUserStatusMes
	var mes message.Message
	mes.Type = message.NotifyUserStatusMesType

	var notifyUserStatusMes message.NotifyUserStatusMes
	notifyUserStatusMes.UserId = userId
	notifyUserStatusMes.Status = message.UserOffline

	// 将notifyUserStatusMes序列化
	data, err := json.Marshal(notifyUserStatusMes)
	if err != nil {
		logHG.ErrInfo("json.Marshal err = ", err)
		return
	}

	// 将序列化后的notifyUserStatusMes赋值给 mes.Data
	mes.Data = string(data)

	// 对mes再次序列化，准备发送
	data, err = json.Marshal(mes)
	if err != nil {
		logHG.ErrInfo("json.Marshal err = ", err)
		return
	}

	// 发送，创建我们Transfer实例，发送
	tf := &utils.Transfer{
		Conn: this.Conn,
	}

	err = tf.WritePkg(data)
	if err != nil {
		logHG.ErrInfo("NotifyMeOnline err = ", err)
		return
	}
}

// 注册服务进程
func (this *UserProcess) ServerProcessRegister(mes *message.Message) (err error) {

	// 1.先从mes中取出 mes.data， 并直接反序列化成Register
	var registerMes message.RegisterMes
	err = json.Unmarshal([]byte(mes.Data), &registerMes)
	if err != nil {
		logHG.ErrInfo("json.Unmarshal fail err = ", err)
		return
	}

	//1.先声明一个resMes
	var resMes message.Message
	resMes.Type = message.RegisterResMesType
	var registerResMes message.RegisterResMes

	// 我们需要到redis数据库去完成注册
	// 1.使用model.MyUserDao 到 redis 去验证
	err = model.MyUserDao.Register(&registerMes.User)

	if err != nil {
		if err == model.ERROR_USER_EXISTS {
			registerResMes.Code = 505
			registerResMes.Error = model.ERROR_USER_EXISTS.Error()
		} else {
			registerResMes.Code = 506
			registerResMes.Error = "注册发生未知错误....."
		}
	} else {
		registerResMes.Code = 200
	}

	data, err := json.Marshal(registerResMes)
	if err != nil {
		logHG.ErrInfo("json.Marshal fail ", err)
	}

	// 4.将data赋值给resMes
	resMes.Data = string(data)

	//5.对resMes进行序列化，准备发送
	data, err = json.Marshal(resMes)
	if err != nil {
		logHG.ErrInfo("json.Marshal fail", err)
		return
	}

	// 6. 发送data，我们将其封装到writePkg函数中
	// 因为使用分层模式（mvc）， 先创建一个Transfer实例，然后读取
	tf := &utils.Transfer{
		Conn: this.Conn,
	}
	err = tf.WritePkg(data)
	return
}

// 编写一个函数serverProcessLogin函数，专门处理登录请求
func (this *UserProcess) ServerProcessLogin(mes *message.Message) (err error) {
	// 1. 先从mes中取出mes.data, 并直接反序列化成LoginMes
	var loginMes message.LoginMes
	err = json.Unmarshal([]byte(mes.Data), &loginMes)
	if err != nil {
		logHG.ErrInfo("json.Unmarshal fail err=", err)
		return
	}

	//先声明一个resMes
	var resMes message.Message
	resMes.Type = message.LoginResMesType
	// 2声明一个LoginResMes,并完成赋值
	var loginResMes message.LoginResMes

	// 我们需要到redis数据库去完成验证
	// 1.使用model.MyUserDao到redis中去验证
	user, err := model.MyUserDao.Login(loginMes.UserId, loginMes.UserPWD)

	if err != nil {
		if err == model.ERROR_USER_NOTEXISTS {
			loginResMes.Code = 500
			loginResMes.Error = err.Error()
		} else if err == model.ERROR_USER_PWD {
			loginResMes.Code = 403
			loginResMes.Error = err.Error()
		} else {
			loginResMes.Code = 505
			loginResMes.Error = "服务器内部错误......"
		}
	} else {
		loginResMes.Code = 200

		// 用户登录成功，把登录成功的放入到userMgr中
		// 将登录成功的用户id赋值给this
		this.UserId = loginMes.UserId
		userMgr.AddOnlineUser(this)
		//通知其他在线用户，我上线了
		this.NotifyOthersOnlineUser(loginMes.UserId)
		// 将当前在线用户的id放入到loginResMes.UserId
		// 遍历userMgr.onLineUsers
		for id, _ := range userMgr.onlineUsers {
			loginResMes.UserId = append(loginResMes.UserId, id)
		}
		logHG.DebugInfo(user, "登录成功....")
	}
	// 3.将loginResMes序列化
	data, err := json.Marshal(loginResMes)
	if err != nil {
		logHG.ErrInfo("json.Marshal fail: ", err)
		return
	}

	// 4.将data赋值给resMes
	resMes.Data = string(data)

	// 5.对resMes进行序列化，准备发送
	data, err = json.Marshal(resMes)
	if err != nil {
		logHG.ErrInfo("json.Marshal fail: ", err)
		return
	}
	// 6.发送data，我们将其封装到writePkg函数
	tf := &utils.Transfer{
		Conn: this.Conn,
	}
	err = tf.WritePkg(data)
	return
}

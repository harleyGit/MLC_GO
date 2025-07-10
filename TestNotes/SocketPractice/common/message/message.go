/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-10 19:01:47
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-10 19:39:56
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/common/message/message.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package message

// 消息类型
const (
	LoginMesType            = "LoginMes"
	LoginResMesType         = "LoginResMes"
	RegisterMesType         = "RegisterMes"
	RegisterResMesType      = "RegisterResMes"
	NotifyUserStatusMesType = "NotifyUserStatusMes"
	SmsMesType              = "SmsMes"
)

// 用户状态长量
const (
	UserOnlie = iota
	UserOffline
	UserBusyStatus
)

// 客户端和服务端之间传输的数据
type Message struct {
	Type string `json: "type"` //消息类型
	Data string `json: "data"` // 消息数据
}

type LoginMes struct {
	UserId   int    `json:"userId"`   //用户id
	UserPWD  string `json:"userPWD"`  //用户密码
	UserName string `json:"userName"` //用户名
}

type LoginResMes struct {
	Code int `json:"code"`//返回状态👵，500表示拥护没有注册，200表示注册成功
	UserId [] int //增加字段，保存用户id的切片
	Error string `json:"error"`//返回错误信息
}

type RegisterMes struct {
	User User `json: "user"` //类型就是User结构体
}

type RegisterResMes struct {
	Code int `json:"code"` //状态码400表示该用户已经占有， 200表示注册成功
	Error string `json:"error"` //返回错误信息
}

type NotifyUserStatusMes struct {
	UserId int `json:"userId"` //用户id
	Status int `json:"status"`//用户状态
}

type SmsMes struct {
	Content string `json:"content"` //内容
	User //匿名结构体继承
}

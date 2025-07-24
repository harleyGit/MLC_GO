/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-23 11:33:05
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-23 11:33:07
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Client/model/curUser.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package model

import (
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"net"
)

// 因为在客户端，我们根据很多地方会使用到curUser,我们将其视为一个全局
type CurUser struct {
	Conn net.Conn
	message.User
}

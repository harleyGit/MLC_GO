/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-21 21:27:38
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-22 09:32:50
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/common/message/process/smsMgr.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package process

import (
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/pkg/logHG"
	"encoding/json"
	"fmt"
)

func outputGroupMes(mes *message.Message) { // 这个地方mes一定SmsMes
	// 1. 反序列化mes.Data 
	var smsMes message.SmsMes
	err := json.Unmarshal([]byte(mes.Data), &smsMes)
	if err != nil {
		logHG.ErrInfo("json.Unmarshal err: ", err.Error())
		return
	}

	//显示信息
	info := fmt.Sprintf("用户id：\t%d 对大家说：\t%s", smsMes.UserId, smsMes.Content)
	logHG.DebugInfo(info)
	logHG.DebugInfo("\n")
}
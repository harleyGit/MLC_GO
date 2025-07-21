/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-21 20:02:59
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-21 20:04:05
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/common/message/process/server.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package process

import "MLC_GO/pkg/logHG"

// 显示登录成功后的界面
func ShowMenu() {
logHG.DebugInfo("----------恭喜xxx登录成功--------")
}
/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-21 16:08:57
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-21 16:12:13
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/command_line_practice/command_line_practice_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package command_line_practice

import "MLC_GO/TestNotes/unfamiliar_grammar_practice/command_line_practice/command_line_practice_v"

func CommandLinePracticeMain() {
	clPracticeV1 := command_line_practice_v.CommandLinePracticeV1{}
	clPracticeV1.ExecutePracticeNone()
	// 加载测试的yaml文件
	clPracticeV1.CommandLinePracticeV1_v1()

}
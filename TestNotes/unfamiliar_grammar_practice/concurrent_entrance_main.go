/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-14 15:55:52
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 16:13:02
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/concurrent_entrance_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package unfamiliar_grammar_practice_package

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	concurrent_program_practice_package "MLC_GO/TestNotes/unfamiliar_grammar_practice/concurrent_program_practice"
	"syscall"
)

func BaseConcurrentProgram_v2_test() {
	concurrent_program_practice_package.BaseConcurrentProgram_v2()
}
func BaseConcurrentProgram_v1_test() {
	concurrent_program_practice_package.BaseConcurrentProgram_v1()
}

func ConcurrentPractice_v1_1() {
	// 创建 program 实例，并取地址传入 svc.Run
	prg := &concurrent_program_practice_package.Program{}

	if err := concurrent_program_practice_package.Run(prg, syscall.SIGINT, syscall.SIGTERM); err != nil {
		logging.ErrInfo("并发编程错误: ",err)
	}
}


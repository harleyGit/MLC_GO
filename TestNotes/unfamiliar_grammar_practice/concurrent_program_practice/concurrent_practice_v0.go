/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-14 16:35:11
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 17:18:24
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/concurrent_program_practice/concurrent_practice_v0.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package concurrent_program_practice

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"time"
)

// 打印有缓冲的通道内部存储数据的数量
func BaseConcurrentProgram_v2 () {
	chel := make(chan int, 6)

	logging.DebugInfo("有缓冲的通道内部存储数据数量为:", len(chel))
	//使用有缓冲的通道发送数据
	chel <- 0
	chel <- 1
	chel <- 2
	chel <- 3
	// 打印有缓冲通道内部存储数据的数量
	logging.DebugInfo("有缓冲通道内部存储数据的数量:", len(chel))
}
// goroutine被调用函数设置参数
func BaseConcurrentProgram_v1 () {
	// 乘客姓名切片
	var offNames = [6]string{"David", "Levon", "Steven", "James", "Tom", "Jack"}
	// 执行并发程序
	go getOff(offNames[:])
	var onNames = [6]string{"张三", "李四", "万物", "糟熘", "周期", "礼拜"}

	for i, name := range onNames {
		logging.DebugInfo("第",i+1,"位乘客", name, "正在上车")
		time.Sleep(1 * time.Second)
	}
}


func getOff(names []string) {
	for i, name := range names {
		logging.DebugInfo("第",i+1,"位乘客", name, "正在下车")
		// 延时一秒
		time.Sleep(1 * time.Second)
	}
}



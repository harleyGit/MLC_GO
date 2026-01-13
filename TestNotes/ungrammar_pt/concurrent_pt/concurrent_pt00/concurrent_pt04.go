/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-06 21:05:24
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:08:08
 * @FilePath: /MLC_GO/TestNotes/ungrammar_pt/concurrent_pt/concurrent_pt00/concurrent_pt04.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package concurrent_pt00

import (
	"MLC_GO/internal/pkg/logHG"
	"fmt"
	"time"
)

/* select解决从管道读取数据阻塞问题 */
func ConcurrentSelect_PT() {

	//1.定义一个管道10个数据int
	intChan := make(chan int, 10)
	for i := 0; i < 10; i++ {
		intChan <- i
	}

	//2.定义一个管5个人数据
	stringChan := make(chan string, 5)
	for i := 0; i < 5; i++ {
		stringChan <- "hello" + fmt.Sprintf("%d", i)
	}

	// 传统在遍历管道时，若不关闭会阻塞导致 deadlock
	// 问题：在实际开发中，可能我们不好确定什么时候关闭管道
	// 通过使用 select 方式可以进行解决
	// label:
	for {
		select {
		// 注意： 这里，若是 intChan一直没有关闭，不会一直阻塞而 deatlock
		// 会自动到下一个case匹配

		case v := <-intChan:
			logHG.DebugFInfo("从 intChan 读取的数据： %d\n", v)
			time.Sleep(time.Second)
		case v := <-stringChan:
			logHG.DebugFInfo("从 stringChan 读取的数据： %d\n", v)
			time.Sleep(time.Second)
		default:
			logHG.DebugInfo("都读取不到，不玩了，程序员可以加入逻辑业余1.。。。。。\n")
			time.Sleep(time.Second)
			return
		}
	}
}

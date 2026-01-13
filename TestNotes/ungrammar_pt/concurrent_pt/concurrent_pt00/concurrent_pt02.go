/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-05 21:04:17
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:07:07
 * @FilePath: /MLC_GO/TestNotes/ungrammar_pt/concurrent_pt/concurrent_pt00/concurrent_pt02.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package concurrent_pt00

import (
	"MLC_GO/internal/pkg/logHG"
	"time"
)

// 大量数字的素数判断
func Concurrent_ShuShuo_PT() {

	intChan := make(chan int, 1000)
	primeChan := make(chan int, 2000)// 放入结果

	//标识退出的管道
	exitChan := make(chan bool, 4) // 4个

	//开启一个协程，向 intChan放入1-8000个数
	go putNum(intChan)

	//开启4个协程，从 intChan 取出数据，并判断是否为素数，如果是
	// 就放入到 primeChan
	for i := 0; i < 4; i++ {
		go primeNum(intChan, primeChan, exitChan)
	}

	// 这里我们主线程，进行处理
	go func()  {
		for i := 0; i < 4; i++ {
			<- exitChan
		}

		// 从exitChan取出4个结果，就可以放心关闭prprimeChan
		close(primeChan)
	}()

	// 遍历primeChan，把结果取出
	for{
		res, ok := <- primeChan
		if !ok{
			break
		}

		// 将结果输出
		logHG.DebugFInfo("素数=%d\n", res)
	}
	logHG.DebugFInfo("main 线程退出")
}

/* 向 intChan 放入1-8000个数 */
func putNum(intChan chan int) {

	for i := 0; i < 8; i++ {
		intChan <- i
	}

	//关闭 intChan
	close(intChan)
}

/* 从 intChan 取出数据，并判断是否为素数
如果是就放入到 printmeChan中
*/

func primeNum(intChan chan int, primeChan chan int, exitChan chan bool) {

	// 使用for循环
	var flag bool
	for {
		time.Sleep(time.Millisecond * 10)
		num, ok := <-intChan
		if !ok { //intChan取不到
			break
		}
		flag = true // 假设是素数
		//判断num是不是素数
		for i := 2; i < num; i++ {
			if num%i == 0 { //hum不是素数
				flag = false
				break
			}
		}

		if flag {
			// 将这个数放入到primeChan
			primeChan <- num
		}
	}

	logHG.DebugInfo("有一个 primeNum 携程因为取不到数据，退出")
	// 这里我们还不能关闭 primeChan
	// 向 exitChan 写入 true
	exitChan <- true
}

/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-06 20:22:53
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-08-06 20:54:55
 * @FilePath: /MLC_GO/TestNotes/ungrammar_pt/concurrent_pt/concurrent_pt00/concurrent_pt03.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package concurrent_pt00

import "MLC_GO/pkg/logHG"

/* 并发只读或者只写 */
func ConcurrentOnlyReadOrWrite() {

	var ch chan int
	ch = make(chan int, 10)
	exitChan := make(chan struct{}, 2)

	go send(ch, exitChan)
	go recv(ch, exitChan)

	var total = 0
	for _ = range exitChan {
		total ++
		if  total == 2 {
			break
		}
	}
	logHG.DebugInfo("结束。。。。。。。。。。")
}

// ch chan<- int,这样chan只能写操作了
func send(ch chan<- int, exitChan chan struct{}) {

	for i := 0; i < 10; i++ {
		ch <- i
	}
	close(ch)

	var a struct{}
	exitChan <- a
}

// ch <-chan int,这样ch只能读操作了
func recv(ch <-chan int, exitChan chan struct{}) {

	for {
		v, ok := <-ch
		if !ok {
			break
		}
		logHG.DebugInfo("读到的数据是：", v)
	}

	var a struct{}
	exitChan <- a
}


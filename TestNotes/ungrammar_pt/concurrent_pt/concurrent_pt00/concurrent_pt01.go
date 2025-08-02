/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-14 11:16:54
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-08-02 21:43:12
 * @FilePath: /MLC_GO/TestNotes/UnfamiliarGrammarPractice/ConcurrentProgramPractice/ConcurrentPracticeV1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package concurrent_pt00

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"
)

// Service 定义了服务应实现的接口
type Service interface {
	start() error
	stop() error
}

// program 实现了 Service 接口，模拟一个长时间运行的服务
type Program struct {
	once sync.Once
	stopCh chan struct{}
}



// Start 启动服务，开启一个后台 goroutine 模拟工作
func (p *Program) start() error {
	logging.DebugInfo("concurrent_practice_v1 服务开始了......")
	
	// 创建一个停止信号通道
	p.stopCh = make(chan struct{})
	go func()  {
		for {
			select {
			case <- p.stopCh:
				logging.DebugInfo("concurrent_practice_v1 服务 goroutine exiting...")
				return
			default:
				// 模拟工作
				logging.DebugInfo("concurrent_practice_v1 服务 running...")
				time.Sleep(1 * time.Second)
			}
		}
	}()
	return nil
}

// Stop 停止服务，通过关闭 stopCh 通知后台 goroutine 退出
func (p *Program) stop() error {
	p.once.Do(func ()  {
		close(p.stopCh)
	})
	logging.DebugInfo("concurrent_practice_v1 服务 停止了")
	return nil
}

// Run 函数接收一个实现了 Service 接口的对象，并监听指定信号，接收到信号后停止服务
func Run(svc Service, signals ...os.Signal) error {
	// 启动服务
	if err := svc.start(); err != nil {
		return err
	}

	// 创建信号接收通道(有缓冲通道)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, signals...)

	// 阻塞等待信号到达
	sig := <- sigCh
	logging.DebugInfo("接受到信号: ", sig)

	// 收到信号后停止服务
	if err := svc.stop(); err != nil {
		return err
	}

	return nil
}

// logFatal 用于输出错误并退出程序
func logFatal(format string, a ...interface{}) {
	fmt.Printf(format, a...)
	os.Exit(1)
}
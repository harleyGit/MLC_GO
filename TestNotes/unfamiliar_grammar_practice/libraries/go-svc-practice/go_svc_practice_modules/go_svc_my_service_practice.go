/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-03 20:29:44
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-03 20:55:43
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/go-svc-practice/go_svc_practice_modules/go_svc_my_service_practice.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package go_svc_practice_modules_package

import (
	"MLC_GO/pkg/logHG"
	"time"

	"github.com/judwhite/go-svc"
)

type myService struct {
	quit chan struct{}
}

// Init、Start、Stop是 go-svc的协议方法
func (s *myService) Init(env svc.Environment) error {

	logHG.DebugInfo("Init： 初始化服务资源")
	s.quit = make(chan struct{})
	return nil
}

func (this *myService) Start() error {

	logHG.DebugInfo("Start: 启动服务")

	go func() {
		for {
			select {
			case <-this.quit:
				logHG.DebugInfo("收到 quit 信号， 停止服务 goroutine")
				return

			default:
				logHG.DebugInfo("服务运行中。。。。。")
				time.Sleep(2 * time.Second)
			}
		}
	}()

	return nil
}

func (this *myService) Stop() error {

	logHG.DebugInfo("Stop: 停止服务")
	close(this.quit) // 通知停止goroutine服务
	return nil
}

func GO_SVC_my_service() {

	service := &myService{}

	// run 会阻塞，直到收到停止信号（如： Cmd+C 或者 SIGTERM）
	if err := svc.Run(service); err != nil {
		logHG.ErrInfo(" 服务出错❌ ", err)
	}
	logHG.DebugInfo("服务已经退出")
}

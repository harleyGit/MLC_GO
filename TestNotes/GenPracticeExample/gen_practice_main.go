/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-23 17:05:53
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 15:51:32
 * @FilePath: /MLC_GO/TestNotes/PracticeGen/practice_gen_test.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gen_practice_example_package

import (
	// "MLC_GO/TestNotes/PracticeGRPCExample/server"
	"MLC_GO/TestNotes/GenPracticeExample/models"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/gredis"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/setting"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/util"
	"MLC_GO/TestNotes/GenPracticeExample/routers"
	"fmt"
	"net/http"
	"os"
	"time"

	// performance_practice_package "MLC_GO/TestNotes/performance_practice"
	"context"
	"os/signal"
	"syscall"

	"github.com/fvbock/endless"
	"github.com/robfig/cron"

	"github.com/gin-gonic/gin"
)

type GenPracticeExample struct {}

// 配置信息,必须优先调用
func (genPracticeEx *GenPracticeExample)setup() {
	setting.Setup()
	models.Setup()
	logging.Setup()
	gredis.Setup()
	util.Setup()
}

//  GenPracticeExample入口函数调试
func GenPracticeMain() {
	genPractice := &GenPracticeExample{}
	genPractice.ExecutePracticeNone()
	// genPractice.setup()

	// genPractice.genPracticeMinor()
	// genPractice.parcticeGenExampleRunV3()
}

// 次要的练习测试
func (genPracticeEx *GenPracticeExample) genPracticeMinor() {
	genPracticeEx.practiceGenPing()
	genPracticeEx.viewCurrentFilePath()
	genPracticeEx.cornPractice()
	genPracticeEx.practiceGenPing()
}
// endless 热更新是采取创建子进程后，将原进程退出的方式，这点不符合守护进程的要求
// http.Server - Shutdown()
// Deprecated: endless库测试
func (genPracticeEx *GenPracticeExample) parcticeGenExampleRunV3() {
	router := routers.InitRouter()

	s := &http.Server{
		Addr: fmt.Sprintf(":%d", setting.ServerSetting.HttpPort),
		Handler:        router, // 设置 HTTP 请求的处理器，这里使用 router（即 Gin 的 Engine）作为请求的处理器。Gin 会根据路由规则处理请求
		ReadTimeout:    setting.ServerSetting.ReadTimeout, // 设置读取请求的超时时间，超过这个时间，连接会被关闭。
		WriteTimeout:   setting.ServerSetting.WriteTimeout, // 设置请求头的最大字节数，这里是 2^20（即 1MB）。如果请求头超过这个大小，会返回 400 Bad Request 错误
		MaxHeaderBytes: 1 << 20,
	}

	// 匿名协程（goroutine）。它启动了一个并发执行的任务，用于监听 HTTP 请求
	go func ()  {
		if err := s.ListenAndServe(); err != nil {
			logging.DebugInfo("Listen: ", err)
		}
	}()

	// 创建一个用于接收操作系统信号的通道 quit
	// os.Signal 是一个特殊类型，表示操作系统的信号（例如 os.Interrupt 和 syscall.SIGTERM）。
	// 通过这个通道，我们可以监听操作系统的中断信号（例如 Ctrl+C 或关闭终端窗口）。
	quit := make(chan os.Signal)
	/* 
	signal.Notify(quit, os.Interrupt): 通过 signal.Notify 函数，将 os.Interrupt 信号（通常是用户按下 Ctrl+C）与 quit 通道进行绑定。
		这样，程序会在收到 os.Interrupt 信号时，将该信号发送到 quit 通道。
		os.Interrupt 是一个操作系统信号，表示请求程序中止（通常来自键盘中断）
	*/
	signal.Notify(quit, os.Interrupt)
	/* 
	<- quit: 程序在这里阻塞，等待从 quit 通道接收到信号。
		当用户按下 Ctrl+C 或程序接收到退出信号时，程序会从 quit 通道接收到信号，继续执行接下来的代码。
	*/
	<- quit

	logging.DebugInfo("Shutdown server ...")

	/* 
	创建一个具有 5 秒钟的超时限制的 context.Context，用来控制优雅关闭过程的时间
		context.Background() 创建一个根上下文（用于无父上下文的场景）。
		context.WithTimeout 创建一个新的上下文，设置一个超时时间。当 5 秒钟到达时，ctx 会自动被取消。
		cancel 是取消函数，调用它会取消上下文并释放相关资源
	*/
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()
	if  err := s.Shutdown(ctx); err != nil {
		logging.ErrInfo("Sever Shutdown:", err)
	}

	logging.DebugInfo("Server exiting")
}

// Deprecated: endless库测试
func (genPracticeEx *GenPracticeExample) parcticeGenExampleRunV2() {

	endless.DefaultReadTimeOut = setting.ServerSetting.ReadTimeout
	endless.DefaultWriteTimeOut = setting.ServerSetting.WriteTimeout
	endless.DefaultMaxHeaderBytes = 1 << 20
	endPoint := fmt.Sprintf(":%d", setting.ServerSetting.HttpPort)

	/* 
	endless.NewServer: 这是 endless 包的函数，它用于创建一个新的 HTTP 服务器。
	与常规的 http.ListenAndServe 不同，endless 提供了对服务器优雅关闭的支持，使得服务器能够在收到停止信号时，等待处理完正在进行的请求后再关闭。
		endPoint: 这是一个表示服务器地址的字符串。通常形式为 ":8080"，表示在本地机器的 8080 端口上监听。
		routers.InitRouter(): 这是调用一个初始化的路由器函数（InitRouter），它返回一个配置好的 HTTP 路由（http.ServeMux 或 *gin.Engine 等），用于处理请求。
	*/
	// 返回一个初始化的 endlessServer 对象，在 BeforeBegin 时输出当前进程的 pid，调用 ListenAndServe 将实际“启动”服务
	server := endless.NewServer(endPoint, routers.InitRouter())
	/* 
	它接收一个字符串参数 add，该参数是服务器绑定的地址（endPoint）。
	在此回调函数中，日志记录了当前进程的 pid（进程ID）。
		syscall.Getpid(): 这个函数返回当前进程的 ID，通常用于调试或日志记录，确认哪个进程正在运行此服务。
	*/
	server.BeforeBegin = func(add string) {
		// 返回当前进程的 ID，通常用于调试或日志记录，确认哪个进程正在运行此服务
		logging.DebugInfo("Actual pid is ", syscall.Getpid())
	}

	// 启动 HTTP 服务器并开始监听请求。
	// ListenAndServe 会监听指定的地址和端口，并使用前面指定的路由器处理传入的请求
	err := server.ListenAndServe()
	if err != nil {
		logging.ErrInfo("Server err: ", err)
	}
}
// Deprecated: parcticeGenExample工程版本V1
func (genPracticeEx *GenPracticeExample) parcticeGenExampleRunV1() {

	gin.SetMode(setting.ServerSetting.RunMode)

	routersInit := routers.InitRouter()
	/*
	readTimeout := setting.ServerSetting.ReadTimeout
	writeTimeout := setting.ServerSetting.WriteTimeout
	endPoint := fmt.Sprintf(":%d", setting.ServerSetting.HTTPPort)//fmt.Sprintf(":%d",8000)//
	maxHeaderBytes := 1 << 20

	server := &http.Server{
		Addr: ":8000",
		Handler: routersInit,
		ReadTimeout: readTimeout,//60 * time.Second,//
		WriteTimeout: writeTimeout,//60 * time.Second ,
		MaxHeaderBytes: maxHeaderBytes,
	}
	*/
	server := &http.Server{
		Addr:           ":8000",//fmt.Sprintf(":%d", setting.HTTPPort), // 设置 HTTP 服务器的监听地址和端口
		Handler:        routersInit, // 设置 HTTP 请求的处理器，这里使用 router（即 Gin 的 Engine）作为请求的处理器。Gin 会根据路由规则处理请求
		ReadTimeout:    setting.ServerSetting.ReadTimeout, // 设置读取请求的超时时间，超过这个时间，连接会被关闭。
		WriteTimeout:   setting.ServerSetting.WriteTimeout, // 设置请求头的最大字节数，这里是 2^20（即 1MB）。如果请求头超过这个大小，会返回 400 Bad Request 错误
		MaxHeaderBytes: 1 << 20,
	}
	logging.Info("🍎 [info] start http server listening %s", server.Addr)//, endPoint

	if err := server.ListenAndServe(); err != nil {
		logging.Error("❌ server failed to start: %v", err)
	}
}

// (PracticeGenExample项目测试)gin测试调用： curl localhost:8000/test
func (genPracticeEx *GenPracticeExample) ginTestFunction() {
	// 返回 Gin 的type Engine struct{...}，里面包含RouterGroup，相当于创建一个路由Handlers，可以后期绑定各类的路由规则和函数、中间件等
	router := gin.Default()
	// 创建不同的 HTTP 方法绑定到Handlers中，也支持 POST、PUT、DELETE、PATCH、OPTIONS、HEAD 等常用的 Restful 方法
	// 当访问服务器上的 /test 路由时，会调用这个匿名函数处理请求
	// gin.Context：Context是gin中的上下文，它允许我们在中间件之间传递变量、管理流、验证 JSON 请求、响应 JSON 请求等，在gin中包含大量Context的方法，例如我们常用的DefaultQuery、Query、DefaultPostForm、PostForm等等
	router.GET("/test", func(ctx *gin.Context) {
		// gin.H{…}：就是一个map[string]interface{}
		ctx.JSON(200, gin.H{
			"message": "test",
		})
	})

	/**
	&http.Server{ ... } 创建了一个 http.Server 类型的指针

	指针的优势：
		内存效率：如果将结构体作为值传递，Go 会复制整个结构体，特别是当结构体较大时，会浪费大量内存和处理时间。通过使用指针，你可以避免复制整个结构体，只传递内存地址。
		修改结构体：使用指针可以直接修改结构体的内容，而不是创建副本。传递指针意味着函数可以修改原始数据，而不是它的副本。对于 http.Server 这样的结构体，你可能需要在程序中修改其字段（如 Addr, Handler, ReadTimeout 等），因此使用指针可以让你修改这些字段，而不需要重新赋值。

		http.Server 需要指针：
			在 Go 中，http.Server 是一个结构体，它包含了许多需要修改的字段（如监听端口、超时设置等）。为了让这些修改生效并且避免不必要的拷贝，通常会传递指针类型的 http.Server。
			ListenAndServe 等方法需要通过指针来调用，因为这些方法可能会改变 Server 结构体的字段，而不是简单地读取它
	*/
	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", setting.ServerSetting.HttpPort), // 设置 HTTP 服务器的监听地址和端口
		Handler:        router, // 设置 HTTP 请求的处理器，这里使用 router（即 Gin 的 Engine）作为请求的处理器。Gin 会根据路由规则处理请求
		ReadTimeout:    setting.ServerSetting.ReadTimeout, // 设置读取请求的超时时间，超过这个时间，连接会被关闭。
		WriteTimeout:   setting.ServerSetting.WriteTimeout, // 设置请求头的最大字节数，这里是 2^20（即 1MB）。如果请求头超过这个大小，会返回 400 Bad Request 错误
		MaxHeaderBytes: 1 << 20,
	}
	// ListenAndServe 是标准库 http.Server 的方法，它启动 HTTP 服务器并开始监听请求
	s.ListenAndServe()
}

// 当前工作目录打印
func (genPracticeEx *GenPracticeExample) viewCurrentFilePath() {
	// 获取当前工作目录
	dir, err := os.Getwd()
	if err != nil {
		logging.ErrInfo("Error getting working directory:", err)
		return
	}
	logging.DebugInfo("3--------在 Go 中使用 os.ReadFile(\"example.txt\") 读取文件时，" + 
	"相对路径是相对于程序的 当前工作目录，当前工作目录路径:", dir)
}

// corn库练习(定时任务调度处理)
func (genPracticeEx *GenPracticeExample) cornPractice() {
	logging.DebugInfo("Corn 开始了.......")

	// 会根据本地时间创建一个新（空白）的 Cron job runner
	c := cron.New()
	// 向 Cron job runner 添加一个 func ，以按给定的时间表运行
	c.AddFunc("* * * * * *", func() {
		logging.DebugInfo("main.go-cornPractice 方法 AddFunc Run models.CleanAllTag...")
		models.CleanAllTag()
	})
	c.AddFunc("* * * * * *", func() {
		logging.DebugInfo("main.go-cornPractice 方法 Run models.CleanAllArticle...")
		models.CleanAllArticle()
	})

	// 在当前执行的程序中启动 Cron 调度程序。其实这里的主体是 goroutine + for + select + timer 的调度控制哦
	c.Start()

	t1 := time.NewTimer(time.Second * 10)
	for {
		select {
		case <-t1.C:
			t1.Reset(time.Second * 10)
		}
	}
}

// gen的ping测试
func (genPracticeEx *GenPracticeExample) practiceGenPing() {//另起一个终端程序，命令： curl 127.0.0.1:8080/ping
	r := gin.Default()
	r.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.Run() // listen and serve on 0.0.0.0:8080
}


// 协议
func (genPracticeExample *GenPracticeExample) ExecutePracticeNone() {}
/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-25 13:47:04
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-02 20:13:40
 * @FilePath: /MLC_GO/main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"MLC_GO/TestNotes/PracticeGenExample/models"
	"MLC_GO/TestNotes/PracticeGenExample/pkg/logging"
	"MLC_GO/TestNotes/PracticeGenExample/pkg/setting"
	"MLC_GO/TestNotes/PracticeGenExample/routers"
	"fmt" //实现了类似 C 语言 printf 和 scanf 的格式化 I/O。格式化动作（‘verb’）源自 C 语言但更简单
	"net/http" //提供了 HTTP 客户端和服务端的实现
	"time"

	"github.com/gin-gonic/gin"
)


func init() {
	setting.Setup()
	models.Setup()
}

func main() {
	// dlvTest()
 	// dlvTest2()
	// dlvThread00()

	// ginTestFunction()

	gin.SetMode(setting.RunMode)

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
		ReadTimeout:    setting.ReadTimeout, // 设置读取请求的超时时间，超过这个时间，连接会被关闭。
		WriteTimeout:   setting.WriteTimeout, // 设置请求头的最大字节数，这里是 2^20（即 1MB）。如果请求头超过这个大小，会返回 400 Bad Request 错误
		MaxHeaderBytes: 1 << 20,
	}
	logging.Info("🍎 [info] start http server listening %s", server.Addr)//, endPoint

	if err := server.ListenAndServe(); err != nil {
		logging.Error("❌ server failed to start: %v", err)
	}
}



// (PracticeGenExample项目测试)gin测试调用： curl localhost:8000/test
func ginTestFunction() {
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
		Addr:           fmt.Sprintf(":%d", setting.HTTPPort), // 设置 HTTP 服务器的监听地址和端口
		Handler:        router, // 设置 HTTP 请求的处理器，这里使用 router（即 Gin 的 Engine）作为请求的处理器。Gin 会根据路由规则处理请求
		ReadTimeout:    setting.ReadTimeout, // 设置读取请求的超时时间，超过这个时间，连接会被关闭。
		WriteTimeout:   setting.WriteTimeout, // 设置请求头的最大字节数，这里是 2^20（即 1MB）。如果请求头超过这个大小，会返回 400 Bad Request 错误
		MaxHeaderBytes: 1 << 20,
	}
	// ListenAndServe 是标准库 http.Server 的方法，它启动 HTTP 服务器并开始监听请求
	s.ListenAndServe()
}

// dlv线程调试
func dlvThread00() {
	for {
		var i int
		var curTime time.Time

		time.Sleep(5 * time.Second)
		i++
		curTime = time.Now()
		fmt.Printf("run %d count, cur time:%v\n", i, curTime)
	}
}

// dlv简单测试 
func dlvTest2() {
	a := 100
	b := 200
	c := Add(a, b)

	fmt.Println("a+b=", c)
}
func Add(v1 int, v2 int) int {
	return  v1 + v2
}

// dlv测试函数
func dlvTest(){
	router := gin.Default()

	router.GET("/welcome", HelloHandler)
	router.Run(":8000")
}
func HelloHandler(c *gin.Context) {
	firstName := c.DefaultQuery("firstname", "Guest")
	lastName := c.Query("lastname")
	c.String(http.StatusOK, "Hello %s %s", firstName, lastName)
}
/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-25 13:47:04
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-04-30 20:52:47
 * @FilePath: /MLC_GO/main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

/*
	推荐放置顺序（标准工程做法）

	func main() {
		// ① 初始化日志（最早）
		initLogger()

		// ② 初始化依赖（DB / Redis / Config）
		initDeps()

		// ③ 构建 router / middleware
		router := buildRouter()

		// ④ 启动 HTTP Server
		startServer(router)
	}

swagger api文档测试： http://127.0.0.1:8000/swagger/index.html
*/
package main

import (
	gen_practice_example_package "MLC_GO/TestNotes/GenPracticeExample"
	"MLC_GO/TestNotes/ungrammar_pt/command_line_practice"
	"MLC_GO/TestNotes/ungrammar_pt/concurrent_pt"
	"MLC_GO/TestNotes/ungrammar_pt/libraries/gin_practice"
	go_svc_practice_main_package "MLC_GO/TestNotes/ungrammar_pt/libraries/go-svc-practice"
	"MLC_GO/TestNotes/ungrammar_pt/libraries/gorm_practice"
	logpt "MLC_GO/TestNotes/ungrammar_pt/log_pt"
	middlewarept "MLC_GO/TestNotes/ungrammar_pt/middleware_pt"
	"MLC_GO/TestNotes/ungrammar_pt/nsq_project_practice"
	"MLC_GO/TestNotes/ungrammar_pt/read_file_practice"
	securitypt "MLC_GO/TestNotes/ungrammar_pt/security_pt"
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

/* 枚举类型 */
type ModuleType string

/* 练习模块值 */
const (
	MLC_Project               ModuleType = "100.00: MLC_GO工程运行"
	Module_middlewareDemo     ModuleType = "16: 中间件调用Demo"
	Security_01               ModuleType = "14.00: 安全：编译或直接运行生成证书（RSA 默认）"
	Security_00_client        ModuleType = "13.02: 启动客户端"
	Security_00_server        ModuleType = "13.01: 启动服务端"
	Security_00_certs         ModuleType = "13.00: 生成证书"
	Module_LOG                ModuleType = "12: 日志错误测试"
	Module_concurrent         ModuleType = "11: 并发测试"
	Module_threadPractice     ModuleType = "10: 线程测试"
	Module_simpleFunction     ModuleType = "9: dlv简单测试"
	Module_dlvFunctionTest    ModuleType = "8: dlv测试函数"
	Module_genPracticeExample ModuleType = "7: GenPracticeExample测试"
	Module_nsqProject         ModuleType = "6: nsq工程中的陌生语法调试"
	Module_gin00              ModuleType = "5: Gin库语法测试"
	Module_gorm00             ModuleType = "4: Gorm库语法测试"
	Module_readFile           ModuleType = "3: 读取文件测试"
	Module_commandLoadConfig  ModuleType = "2: 命令行加载配置文件"
	Module_go_svc             ModuleType = "1: go_svc轻量库使用"
)

func getPracticeModules() []ModuleType {

	return []ModuleType{
		MLC_Project,
		Module_middlewareDemo,
		Security_01,
		Security_00_client,
		Security_00_server,
		Security_00_certs,
		Module_LOG,
		Module_concurrent,
		Module_threadPractice,
		Module_simpleFunction,
		Module_dlvFunctionTest,
		Module_genPracticeExample,
		Module_nsqProject,
		Module_gin00,
		Module_gorm00,
		Module_readFile,
		Module_commandLoadConfig,
		Module_go_svc,
	}
}

func main() {

	practiceKnowledge()
}

func practiceKnowledge() {
	modules := getPracticeModules()
	// 按行读取用户输入，避免 Scanf 在换行匹配时产生额外阻塞。
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Println("\n\n=====================👏欢迎练习测试陌生知识点==========================")
		for _, module := range modules {
			fmt.Printf("\t\t\t 🍎 %s\n", module)
		}
		fmt.Printf("请输入序号进入对应功能（输入 q 退出）：\n> ")

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Printf("读取输入失败: %v\n", err)
			}
			return
		}

		functionModule := strings.TrimSpace(scanner.Text())
		if functionModule == "q" || functionModule == "quit" || functionModule == "exit" {
			fmt.Println("已退出练习菜单")
			return
		}

		if !runPracticeModule(functionModule) {
			fmt.Printf("无效序号：%s，请重新输入。\n", functionModule)
		}
	}
}

// runPracticeModule 负责把菜单输入映射到对应模块执行，返回 false 表示输入未命中任何功能。
func runPracticeModule(functionModule string) bool {
	switch functionModule {
	case "100", "100.00":
		mlc_main()
	case "16", "16.00":
		middlewarept.MiddlewareDemoMain()
	case "14", "14.00":
		securitypt.SecurityV01_mtls_tool()
	case "13.02":
		securitypt.SecurityV00_activate_Client()
	case "13.01":
		securitypt.SecurityV00_activate_Server()
	case "13", "13.00":
		securitypt.SecurityV00_generate_certs()
	case "12", "12.00":
		logpt.LogMainPT()
	case "11", "11.00":
		concurrent_pt.ConcurrentPTMain()
	case "10", "10.00":
		dlvThread00()
	case "9", "9.00":
		dlvTest2()
	case "8", "8.00":
		dlvTest()
	case "7", "7.00":
		// GenPracticeExample测试
		gen_practice_example_package.GenPracticeMain()
	case "6", "6.00":
		// nsq工程中的陌生语法调试
		nsq_project_practice.NSQProjectPracticeMain()
	case "5", "5.00":
		// Gin库语法测试
		gin_practice.GinPracticeMain()
	case "4", "4.00":
		// Gorm库语法测试
		gorm_practice.GormPracticeMain()
	case "3", "3.00":
		// 读取文件测试
		read_file_practice.ReadFilePracticeMain()
	case "2", "2.00":
		// 命令行加载配置文件
		command_line_practice.CommandLinePracticeMain()
	case "1", "1.00":
		go_svc_practice_main_package.Go_SVC_Practice_Main()
	default:
		return false
	}

	return true
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
	return v1 + v2
}

// dlv测试函数
func dlvTest() {
	router := gin.Default()

	router.GET("/welcome", HelloHandler)
	router.Run(":8000")
}
func HelloHandler(c *gin.Context) {
	firstName := c.DefaultQuery("firstname", "Guest")
	lastName := c.Query("lastname")
	c.String(http.StatusOK, "Hello %s %s", firstName, lastName)
}

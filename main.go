/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-25 13:47:04
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-17 20:28:45
 * @FilePath: /MLC_GO/main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

/*
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
	"MLC_GO/TestNotes/ungrammar_pt/nsq_project_practice"
	"MLC_GO/TestNotes/ungrammar_pt/read_file_practice"
	securitypt "MLC_GO/TestNotes/ungrammar_pt/security_pt"
	"MLC_GO/internal/config"
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	UserhandlerPackage "MLC_GO/internal/modules/user/handler"
	"MLC_GO/internal/pkg/logHG"
	"fmt" //实现了类似 C 语言 printf 和 scanf 的格式化 I/O。格式化动作（‘verb’）源自 C 语言但更简单
	"log"
	"net/http" //提供了 HTTP 客户端和服务端的实现
	"time"

	"github.com/gin-gonic/gin"
)

/* 枚举类型 */
type ModuleType string

/* 练习模块值 */
const (
	MLC_Project ModuleType = "100.00: MLC_GO工程运行"

	Security_01 ModuleType = "14.00: 安全：编译或直接运行生成证书（RSA 默认）"
	Security_00_certs ModuleType = "13.00: 生成证书"
	Security_00_server ModuleType = "13.01: 启动服务端"
	Security_00_client ModuleType = "13.02: 启动客户端"
	Module_LOG                ModuleType = "12: 日志错误测试"
	Module_go_svc             ModuleType = "1: go_svc轻量库使用"
	Module_commandLoadConfig  ModuleType = "2: 命令行加载配置文件"
	Module_readFile           ModuleType = "3: 读取文件测试"
	Module_gorm00             ModuleType = "4: Gorm库语法测试"
	Module_gin00              ModuleType = "5: Gin库语法测试"
	Module_nsqProject         ModuleType = "6: nsq工程中的陌生语法调试"
	Module_genPracticeExample ModuleType = "7: GenPracticeExample测试"
	Module_dlvFunctionTest    ModuleType = "8: dlv测试函数"
	Module_simpleFunction     ModuleType = "9: dlv简单测试"
	Module_threadPractice     ModuleType = "10: 线程测试"
	Module_concurrent         ModuleType = "11: 并发测试"
)

func init() {
	PersistenceSQLPackage.LoadSQLEnvValue()
}
func getPracticeModules() []ModuleType {

	return []ModuleType{
		MLC_Project,

		Security_01,
		Security_00_certs,
		Security_00_server,
		Security_00_client,
		Module_LOG,
		Module_go_svc,
		Module_commandLoadConfig,
		Module_readFile,
		Module_gorm00,
		Module_gin00,
		Module_nsqProject,
		Module_genPracticeExample,
		Module_dlvFunctionTest,
		Module_simpleFunction,
		Module_threadPractice,
		Module_concurrent,
	}
}

func main() {

	practiceKnowledge()
}

func mlc_main() {
	logHG.DebugInfo("MLC_GO项目启动中...")
	UserhandlerPackage.RegisterUserRoutesV2()
	srv := http.Server{
		Addr: ":8080",
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logHG.DebugInfo("Starting server on :8080")
	log.Fatal(srv.ListenAndServe()) 

	env := config.GetEnv()
	if err := config.LoadConfig(string(env)); err != nil {
		logHG.FatalFInfo("加载配置文件失败: %v\n", err)
		return		
	}
	logHG.DebugInfo("当前环境: %s\n", env)
}

func practiceKnowledge() {

	modules := getPracticeModules()
	for true {
		fmt.Println("\n\n=====================👏欢迎练习测试陌生知识点==========================")
		for _, module := range modules {
			fmt.Printf("\t\t\t 🍎 %s\n", module)
		}
		fmt.Printf("请输入序号进入对应功能：\n\n")

		var functionModule float64
		fmt.Scanf("%f\n\n", &functionModule)

		switch functionModule {
			case 100.00:
			mlc_main()
		case 14:
			securitypt.SecurityV01_mtls_tool()
		case 13.00: // 支持小数匹配（带容差，避免浮点精度误差）case math.Abs(functionModule-13.01) < 1e-6
			securitypt.SecurityV00_generate_certs()
		case 13.01:
			securitypt.SecurityV00_activate_Server()
		case 13.02:
			securitypt.SecurityV00_activate_Client()
		case 12: // 支持整数匹配 int(functionModule) == 12
			logpt.LogMainPT()
		case 1:
			go_svc_practice_main_package.Go_SVC_Practice_Main()
		case 2:
			// 命令行加载配置文件
			command_line_practice.CommandLinePracticeMain()
		case 3:
			// 读取文件测试
			read_file_practice.ReadFilePracticeMain()
		case 4:
			// Gorm库语法测试
			gorm_practice.GormPracticeMain()
		case 5:
			// Gin库语法测试
			gin_practice.GinPracticeMain()
		case 6:
			// nsq工程中的陌生语法调试
			nsq_project_practice.NSQProjectPracticeMain()
		case 7:
			//GenPracticeExample测试
			gen_practice_example_package.GenPracticeMain()
		case 8:
			dlvTest()
		case 9:
			dlvTest2()
		case 10:
			dlvThread00()
		case 11:
			concurrent_pt.ConcurrentPTMain()
		}
	}
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

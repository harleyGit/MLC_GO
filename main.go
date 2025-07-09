/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-25 13:47:04
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-09 20:05:12
 * @FilePath: /MLC_GO/main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

/*
swagger api文档测试： http://127.0.0.1:8000/swagger/index.html
*/
package main

import (
	gen_practice_example_package "MLC_GO/TestNotes/GenPracticeExample"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/command_line_practice"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gin_practice"
	go_svc_practice_main_package "MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/go-svc-practice"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/nsq_project_practice"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/read_file_practice"
	"fmt"      //实现了类似 C 语言 printf 和 scanf 的格式化 I/O。格式化动作（‘verb’）源自 C 语言但更简单
	"net/http" //提供了 HTTP 客户端和服务端的实现
	"time"
	"github.com/gin-gonic/gin"
)

/* 枚举类型 */
type ModuleType string

/* 练习模块值 */
const (
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
)

func getPracticeModules() []ModuleType {

	return []ModuleType{
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
	}
}

func main() {

	practiceKnowledge()
}

func practiceKnowledge() {

	modules := getPracticeModules()
	for true {
		fmt.Println("=====================👏欢迎练习测试陌生知识点==========================")
		for _, module := range modules {
			fmt.Printf("\t\t\t 🍎 %s\n", module)
		}
		fmt.Printf("请输入序号进入对应功能：\n\n")

		var functionModule int
		fmt.Scanf("%d\n", &functionModule)

		switch functionModule {
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

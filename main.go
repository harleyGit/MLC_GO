/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-25 13:47:04
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 15:51:53
 * @FilePath: /MLC_GO/main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

/*
swagger api文档测试： http://127.0.0.1:8000/swagger/index.html
*/
package main

import (
	gen_practice_example_package "MLC_GO/TestNotes/GenPracticeExample"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gin_practice"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/nsq_project_practice"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/read_file_practice"

	"fmt"      //实现了类似 C 语言 printf 和 scanf 的格式化 I/O。格式化动作（‘verb’）源自 C 语言但更简单
	"net/http" //提供了 HTTP 客户端和服务端的实现
	"time"

	"github.com/gin-gonic/gin"
)

func main() {

	practiceTestMethod()

	
}

// 测试方法
func practiceTestMethod() {

	// 读取文件测试
	read_file_practice.ReadFilePracticeMain()

	return
	// Gorm库语法测试
	gorm_practice.GormPracticeMain()
	
	// Gin库语法测试
	gin_practice.GinPracticeMain()
	
	// nsq工程中的陌生语法调试
	nsq_project_practice.NSQProjectPracticeMain()
	
	//GenPracticeExample测试
	gen_practice_example_package.GenPracticeMain()
	// dlvTest()
 	// dlvTest2()
	// dlvThread00()	
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
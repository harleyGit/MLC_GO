/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-25 13:47:04
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-02-25 15:06:48
 * @FilePath: /MLC_GO/main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)


func init() {

}

func main() {
	// dlvTest()
 	dlvTest2()
}


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
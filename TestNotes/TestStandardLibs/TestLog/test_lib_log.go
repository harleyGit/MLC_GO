/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-17 13:30:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-17 13:34:43
 * @FilePath: /MLC_GO/TestNotes/TestStandardLibs/TestLog/test_lib_log.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import(
	"fmt"
	"log"
)

func main() {
	defer fmt.Println("发生了 panic 错误！")
	log.Print("my log")
	log.Printf("my log %d", 404)
	
	name := "David"
	age := 26
	log.Println(name, ":", age)
	log.Panic("致命错误！")
}


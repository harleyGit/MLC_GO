/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-26 10:14:40
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-26 10:39:07
 * @FilePath: /MLC_GO/TestNotes/TestTemplateEngine/test_template_engine.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"html/template"
	"log"
	"os"
)

type Detail struct {
	Class string
	Title string
	Content string
}

func main() {
	// 原生HTML模版测试
	testOriginalHTMLTemplate()
}

// 原生HTML模版测试
func testOriginalHTMLTemplate() {
	pwd, _ := os.Getwd()
	t, err := template.ParseFiles(pwd + "/index0.html")
	if err != nil {
		log.Println(err)
		return
	}

	var detail Detail
	detail = Detail{
		Class: "test-class",
		Title: "test-title",
		Content: "test-content",
	}

	err = t.Execute(os.Stdout, &detail)
	if err != nil {
		log.Println(err)
		return
	}
}
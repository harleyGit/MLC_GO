/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-17 12:00:05
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-17 12:10:23
 * @FilePath: /MLC_GO/TestNotes/TestStandardLibs/TestLib/test_lib_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"fmt"
	"os"
)

// 创建文件
func createFile() {
	f, err := os.Create("test02.txt")
	if err != nil {
		fmt.Printf("err: %v\n", err)
	} else {
		fmt.Printf("f: %v\n", f)
	}
}

// 重命名文件
func renameFile() {
	err := os.Rename("test.txt", "test01.txt")
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
}

// 写文件
func writeFile() {
	s := "hello wrold"
	os.WriteFile("test03.txt", []byte(s), os.ModePerm)
}

func main() {
	createFile()
	renameFile()
	writeFile()
}

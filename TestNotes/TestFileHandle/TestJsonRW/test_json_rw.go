/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-22 15:52:40
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-22 16:04:21
 * @FilePath: /MLC_GO/TestNotes/TestFileHandle/TestJsonRW/test_json_rw.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

// / 二进制文件的读写
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type PersonInfo struct{
	Name string
	Age int
	Sex string
}

func main() {
	// json写入
	testJsonWrite()
	// json文件读取
	testJsonRead()
}

func testJsonRead() {
	// 创建文件
	filePtr, err := os.Open("info.json")
	if err != nil {
		fmt.Println("文件打开失败！[Err: %s]", err.Error())
		return
	}
	defer filePtr.Close()

	var info []PersonInfo
	// 创建json解码器
	decoder := json.NewDecoder(filePtr)
	err = decoder.Decode(&info)
	if err != nil {
		fmt.Println("解码失败", err.Error())
	}else {
		fmt.Println("解码成功")
		fmt.Println(info)
	}
}


func testJsonWrite() {
	info := []PersonInfo{{"Dave", 29, "Male"}, {"Leon", 32, "Male"}}

	// 创建文件
	filePtr, err := os.Create("info.json")
	if err != nil {
		fmt.Println("文件创建失败！", err.Error())
		return
	}
	defer filePtr.Close()

	// 创建json编码器
	encoder := json.NewEncoder(filePtr)

	err = encoder.Encode(info)
	if err != nil {
		fmt.Println("编码错误", err.Error())
	}else {
		fmt.Println("编码成功")
	}
}

/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-22 15:29:12
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-22 15:47:52
 * @FilePath: /MLC_GO/TestNotes/TestFileHandle/TestBinaryReadAndWrite/test_binary_rw.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"os"
)


func main() {
	// 二进制文件并写入
	// testBinaryWrite()


	// 自定义二进制写入
	testCustomBinaryWrite()
}

type Website struct {
	Url int32
}
// 自定义二进制写入
func testCustomBinaryWrite() {
	file, err := os.Create("output.bin")
	for i := 1; i <= 10; i++ {
		info := Website{ int32(i) }
		if err != nil {
			fmt.Println("文件创建失败", err.Error())
			return
		}

		defer file.Close()

		var bin_buf bytes.Buffer
		binary.Write(&bin_buf, binary.LittleEndian, info)
		b := bin_buf.Bytes()
		_,err = file.Write(b)
		if err != nil {
			fmt.Println("编码失败", err.Error())
			return
		}
	}
	fmt.Println("编码成功")
}

// 二进制文件并写入
func testBinaryWrite() {
	info := "黄沙百战穿金甲，不破楼兰终不还。"
	file, err := os.Create("output.gob")
	if err != nil {
		fmt.Println("创建文件失败", err.Error())
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(info)
	if err != nil {
		fmt.Println("编码错误", err.Error())
	}else {
		fmt.Println("编码成功")
	}
}
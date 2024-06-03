/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2024-05-22 19:48:47
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2024-05-22 20:05:25
 * @FilePath: /MLC_GO/TestNotes/TestError.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"errors"
	"fmt"
)

func ErrorUsage() {
	err := errors.New("type")
	if err != nil {
		fmt.Println(err.Error())
	}

	err2 := fmt.Errorf("err: %s", "found 2")
	if err2 != nil {
		fmt.Println(err2.Error())
	}
}

func main() {
	ErrorUsage()
}

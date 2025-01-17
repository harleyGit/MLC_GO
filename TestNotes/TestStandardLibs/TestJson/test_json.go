/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-17 13:53:32
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-17 14:00:52
 * @FilePath: /MLC_GO/TestNotes/TestStandardLibs/TestJson/test_json.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%Ap
 */

package main

import(
	"encoding/json"
	"fmt"
)

type Person struct {
	Name string
	Age int
	Sex string
}

// 字符串转换成届构体
func Unmarshal() {
	b1 := []byte(`{"Name":"David", "Age": 26, "Sex": "Male"}`)
	var m Person
	json.Unmarshal(b1, &m)
	fmt.Printf("m: %v\n", m)
}

// 结构体转换成字符串
func Marshal() {
	p := Person{
		Name: "Lisa滤镜",
		Age: 34,
		Sex: "Male",
	}
	b,_ := json.Marshal(p)
	fmt.Printf("b: %v\n", string(b))
}

func main() {
	Unmarshal()

	Marshal()
}
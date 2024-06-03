/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2024-05-10 11:35:32
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2024-06-03 09:42:22
 * @FilePath: /GoProject/MLC_GO/hellow.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"fmt"
	"reflect"
)

// 定义全局变量
var(
    globalVar00 = 100
    globalVar01 = 200
    globalVar02 = "全局变量V300"
)

func main() {
	fmt.Println("🍎 我创建了一个简单 hello, world")

	testVariable00()
    testVariable01()
}


func testVariable01(){
    type Info struct {
        Name string `json:"name"`
        Age int `json:"name"`
        Number int `json:"number"`
    }
    var info Info
    info.Name = "李雪飞"
    info.Age = 20
    info.Number = 100

    var typeInfo reflect.Type
    typeInfo = reflect.TypeOf(info)
    if _, ok := typeInfo.FieldByName("Name"); ok {
       fmt.Println("BOOL值测试--含有字段Name：",ok )
    }else{
        fmt.Println("BOOL值测试--不含有字段Name：",ok )
    }
}

func testVariable00(){//测试变量

    fmt.Println("\n<===================测试变量===================>")

    //省略var， := 左边的变量不应该是自己声明过的，否则会导致编译出错
    name00 := "你好啊 我创建了一个变量1111222"
	fmt.Println(name00)

    //第一种方式：一次性声明多个变量
    var n1, n2, n3 int
    fmt.Println("\nn1=", n1, "n2=", n2, "n3=", n3)

    //第2种方式：一次性声明多个变量
    var n4, name, n5 = 100, "Tom", 888
    fmt.Println("\nn4=", n4, "name=", name, "n5=", n5)

    //第3种方式：一次性声明多个变量,使用类型推倒
    n6, name01, n7 := 100, "Tom🍎", 888
    fmt.Println("\nn6=", n6, "name01=", name01, "n7=", n7)

    fmt.Println("\nglobalVar00=", globalVar00, "globalVar01=", globalVar01, "globalVar02=", globalVar02)


}

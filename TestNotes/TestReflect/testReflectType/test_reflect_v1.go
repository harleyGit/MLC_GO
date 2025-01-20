/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-19 20:24:22
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-20 20:55:07
 * @FilePath: /MLC_GO/TestNotes/TestReflect/test_reflect_v1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

 /// 反射的类型与种类
package main

import(
	"fmt"
	"reflect"
)

type myint int

type Book struct {	// 定义新类型Book， 其数据类型是struct
	BookName string
}

func main() {
	var i int = 711 // 定义 int 类型的变量i
	typel := reflect.TypeOf(i)	// 创建变量i地反射对象
	// 打印反射对象typel的类型和种类
	fmt.Println("反射对象typel的类型：", typel.Name())
	fmt.Println("反射对象typel的种类：", typel.Kind())
	fmt.Println()

	var mi myint = 44	// 定义 myint 类型的变量mi
	typeMI := reflect.TypeOf(mi)	// 创建变量mi的反射对象
	// 打印反射对象 typeMI 的类型和种类
	fmt.Println("反射对象 typeMI 的类型：", typeMI.Name())
	fmt.Println("反射对象 typeMI 的种类：", typeMI.Kind())
	fmt.Println()

	book := Book{BookName: " GOlang 语言从入门到精通" } // 定义Book类型的变量book
	typeBook := reflect.TypeOf(book)	// 创建变量book的反射对象
	// 打印反射对象 typeBook 的类型和种类
	fmt.Println("反射对象 typeBook 的类型：", typeBook.Name())
	fmt.Println("反射对象 typeBook 的种类：", typeBook.Kind())
}


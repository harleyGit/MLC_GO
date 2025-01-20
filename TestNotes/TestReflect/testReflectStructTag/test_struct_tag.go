

/// 结构体标签的获取
package main

import(
	"fmt"
	"reflect"
)

type Book struct {	// 定义新类型Book， 其数据类型是struct
	BookName string `json: "name"`
	BookPrice float64 `json: "price"`
}

func main() {
	book := reflect.TypeOf(Book{})	// 创建结构体Book{} 的反射对象
	nm,_ := book.FieldByName("BookName")	//获取字段BookName的信息
	fmt.Println("字段BookNam的信息： \n", nm)
	fmt.Println()

	tag := nm.Tag	// 获取与字段BookName对应结构体标签
	fmt.Println("与字段BookName对应的结构体标签： \n", tag)
	value,_ := tag.Lookup("json")	//获取与字段 BookName对应的结构体标签中的键值对
	fmt.Println("与字段 BookName 对应的结构体标签中的键值对：\n key: json, value: ", value)
}

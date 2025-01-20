/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-19 20:13:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-19 20:23:54
 * @FilePath: /MLC_GO/TestNotes/TestReflect/test_reflect_v0.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
 /// 反射值的修改
package main

import(
	"fmt"
	"reflect"
)

func main() {
	var f float64 = 3.1415926
	valueF := reflect.ValueOf(&f).Elem()	// 获取指针指向的反射对象的值
	// 打印反射对象valueF 是否具有 “可写性”
	fmt.Println("反射对象 valueF 是否具有 “可写性”：", valueF.CanSet())
	fmt.Println("反射对象 valueF 的初始值：", valueF) // 打印反射对象 valueF 的初始值
	valueF.SetFloat(3.14)	// 把反射对象的值修改为3.14
	fmt.Println("反射对象 valueF 的初始值被修改为：", valueF) // 打印反射对象 valueF 的修改后的值
}
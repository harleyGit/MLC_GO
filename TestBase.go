/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2024-05-10 11:35:32
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2024-06-04 20:13:52
 * @FilePath: /GoProject/MLC_GO/hellow.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"
)

// 定义全局变量
var (
	globalVar00 = 100
	globalVar01 = 200
	globalVar02 = "全局变量V300"
)

func main() {
	fmt.Println("🍎 我创建了一个简单 hello, world")

	//testVariable00()
	//testVariable01()

	//testArrayAndSlice()

	//testDictionary()

	//testStruct()

    testInterface()
}


type error interface {
    Error() string
}
func testInterface(){//接口
    var ErrExampleNew = errors.New("你好 🌍世界 error")
    var ErrExampleFmt = fmt.Errorf("你好 🌍世界, 格式化： %s", "error")

    fmt.Println(reflect.TypeOf(ErrExampleNew),reflect.TypeOf(ErrExampleFmt))
}



// 结构体可以绑定相应的方法。
// 结构体的字段和方法是否可以访问需要根据字段和方法首字母的大小写来确定，大写表示可访问（公有），而小写表示私有。
func testStruct() { //结构体
    /**
    结构体在Go语言中是不同数据类型的集合，包含字段和方法。方法和函数的区别在于，方法绑定给了对象，即结构体类型，而函数是代码块的封装。
    结构体能够以不同的组合继承相应结构体的字段和方法。
    匿名字段的主结构体可以自动拥有字段和方法。结构体初始化时会分配一段连续的内存地址。
    */
	fmt.Println("\n<===================结构体===================>")

	type Info struct {
		Name string
		_    int // _ 表示占位符
		Age  int
	}

	var infoOne Info = Info{ //建议使用方法1（infoOne），即以命名方式进行初始化操作，因为这样的话就可以不考虑字段的顺序进行赋值，而且更容易理解。
		Name: "解晓东",
		Age:  23,
	}
	var infoTwo = Info{"望着🔥荣安要", 2000, 12}
	var infoThree = new(Info)
	infoThree = &Info{
		Name: "🍎🍎",
		Age:  20,
	}

	fmt.Println("one", infoOne)
	fmt.Println("Two", infoTwo)
	fmt.Println("Three", *infoThree)

	//结构体初始化操作，分配一段连续的内存地址，结构体占用空间大小等于各属性占用空间大小之和（24=16+8）。
	fmt.Println(
		"\n",
		"infoOne大小：", unsafe.Sizeof(infoOne),
		fmt.Sprintf("\ninfoOne.Name地址：%x \n- infoOne.Name大小：%d \n- infoOne.Age地址：%x \n- infoOne.Age大小：%d",
			&infoOne.Name,
			unsafe.Sizeof(infoOne.Name),
			&infoOne.Age,
			unsafe.Sizeof(infoOne.Age)))

	//匿名字段
	type University struct {
		Name     string
		Location string
	}
	type Student struct {
		Name string
		University//匿名字段为University
	}

    //匿名字段具有和主结构体相同的字段Name，初始化赋值时需要采用多层级“.”的形式来引用，比如std.University.Name="ShangHai"，以这种方式可以直接赋值。
	var std Student
	std.Name = "逻辑思维"
	std.University.Name = "匿名字段-布谷鸟"
	std.Location = "南极大陆"
	fmt.Println("\n",std)

}

// /map是引用类型，使用make初始化。
// /无序：输出键的顺序和定义顺序不一致。
func testDictionary() { //字典
	fmt.Println("\n<===================字典===================>")

	var onMap = func(name map[string]int) {
		for key, value := range name {
			fmt.Println("字典key：", key, ",值：", value)
		}

		//赋值
		name["Life"] = 100
		//判断是否存在key： Go
		if value1, ok := name["Go"]; ok {
			fmt.Println("值是：", value1)
		} else {
			fmt.Println("no exits Go")
		}

		//删除key： java
		delete(name, "java")
	}

	nameMap := make(map[string]int)
	nameMap["java"] = 200
	nameMap["php"] = 100
	nameMap["python"] = 180
	nameMap["JavaScript"] = 220
	onMap(nameMap)
}

// /数组和切片的操作几乎相同，区别在于数组是固定长度的，而切片可以扩充容量
func testArrayAndSlice() { //切片和数组
	fmt.Println("\n<===================切片和数组===================>")

	var opList = func(number [4]int) {
		fmt.Println(number[1], "类型：", reflect.TypeOf(number[1]))
		fmt.Println("数组长度：", len(number))
		fmt.Println("数组和类型：", number[1:], reflect.TypeOf(number[1:]))

		//数组遍历1
		for index, one := range number {
			fmt.Println("数组遍历方式一: ", index, one)
		}
		//
		for i := 0; i < len(number); i++ {
			fmt.Println("数组遍历方式二:", i, number[i])
		}
	}

	//数组
	var number [4]int = [...]int{1, 2, 3, 4}
	opList(number)

	var opSlice = func(name []string) []string {
		fmt.Println("\n切片第一个元素：", name[1], "类型：", reflect.TypeOf(name[1]))

		for index, one := range name {
			fmt.Println("切片遍历方式一：", index, one)
		}

		name = append(name, "姜子牙")
		return name
	}
	//切片
	//切片是引用类型，所以对切片初始化时可以采用显式的方式对切片赋值，也可以使用make关键字
	var name []string = []string{"GO", "Python", "Java", "C++", "C#"}
	fmt.Println(opSlice(name))

}

func testVariable01() {
	type Info struct {
		Name   string `json:"name"`
		Age    int    `json:"name"`
		Number int    `json:"number"`
	}
	var info Info
	info.Name = "李雪飞"
	info.Age = 20
	info.Number = 100

	var typeInfo reflect.Type
	typeInfo = reflect.TypeOf(info)
	if _, ok := typeInfo.FieldByName("Name"); ok {
		fmt.Println("BOOL值测试--含有字段Name：", ok)
	} else {
		fmt.Println("BOOL值测试--不含有字段Name：", ok)
	}
}

func testVariable00() { //测试变量

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

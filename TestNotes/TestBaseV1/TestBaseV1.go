/*
 * @title: 学习测试
 * @Author: gang.huang
 * @Date: 2021-08-20 22:51:45
 * @LastEditTime: 2024-06-14 17:20:07
 * @FilePath: /GoDemo/main.go
 */

package main

import (
	"bytes"
	"container/list"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"reflect"
	"runtime"
	"sync/atomic"
)

func main() {
	fmt.Printf("<=============== 🍎 🍎 🍎 ===============> \n\n")
	//fmt.Println("🍎 welcome to Go Lang! 🍎 ")

	init_array()//测试数组
	// point_flag()
	//chargeValue()
	//pointTest1()
	//lowVar()
	// section_test()
	//multiplication_table()
	// list_delete()
	// paramTranslate()
	// testAnonymousFunction()
	// testAnnoymousFunction1()
	// testFuncImplInterface()
	// testFuncImplInterface1()
	// testClosure1_1()
	// testVariableParameters("hammer", " mom", " and", " hawk")
	// testDef()
	// testError("💣 ❌ 错误测试")
	// fmt.Println(testError1(1, 0))
	// testError2()
	// panic("💣 ❌  崩溃 ")
	// testPanic()
	// testPanic1()

	/*
		var version int = 1
		cmd := testStruct(
			"version",
			&version,
			"show version",
		)
		fmt.Println("取地址结构体 cmd：", cmd)
	*/
	// testFuncMethod()
	// testFuncMethod1()
	// testStruct1()
	// testStruct2()
	// testStruct3()
	// testInterface1()
	// testInterface2()
	// testInterface3()
	// testInterface4()
	// testInterface5()
	// testInterface6()

	// testGoroutine1()

	testLock1()

	fmt.Printf("\n\n<=============== 🍑 🍑 🍑 ===============> ")

}


/**
 * @description: 序列号生成器
 */

var (
	// 序列号
	seq int64
)

func testLock1() {
	//生成10个并发序列号
	for i := 0; i < 10; i++ {
		go GenID()
	}
	fmt.Println(GenID())
}
func GenID() int64 {
	// 尝试原子的增加序列号
	// 使用原子操作函数atomic.Add Int64()对seq()函数加1操作。
	// 不过这里故意没有使用atomic.Add Int64()的返回值作为Gen ID()函数的返回值，因此会造成一个竞态问题
	// atomic.AddInt64(&seq, 1)
	// return seq

	// 尝试原子的增加序列号
	return atomic.AddInt64(&seq, 1)
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

//<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

func testGoroutine1() {
	fmt.Println("CPU线程数量: ", runtime.NumCPU())

	runtime.GOMAXPROCS(runtime.NumCPU())
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

//<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
/**
 * @description: 状态接口
 */
type State interface {
	// 获取状态名字
	Name() string
	// 该状态是否允许同状态转移
	EnableSameTransit() bool
	// 响应状态开始时
	OnBegin()
	// 响应状态结束时
	OnEnd()
	// 判断能否转移到某个状态
	CanTransitTo(name string) bool
}

// 从状态实例获取状态名
func StateName(s State) string {
	if s == nil {
		return "none"
	}
	// 使用反射获取状态的名称
	return reflect.TypeOf(s).Elem().Name()
}

/**
 * @description: 状态基本信息
 */
// 状态的基础信息和默认实现
type StateInfo struct {
	// 状态名
	name string
}

// 状态名
func (s *StateInfo) Name() string {
	return s.name
}

// 提供给内部设置名字
// setName()方法的首字母小写，表示这个方法只能在同包内被调用。
// 这里我们希望setName()不能被使用者在状态初始化后随意修改名称，而是通过后面提到的状态管理器自动赋值
func (s *StateInfo) setName(name string) {
	s.name = name
}

// 允许同状态转移
func (s *StateInfo) EnableSameTransit() bool {
	return false
}

// 默认将状态开启时实现
func (s *StateInfo) OnBegin() {

}

// 默认将状态结束时实现
func (s *StateInfo) OnEnd() {}

// 默认可以转移到任何状态
func (s *StateInfo) CanTransitTo(name string) bool {
	return true
}

/**
 * @description: 状态管理器
 */

type StateManager struct {
	// 已经添加的状态
	// 声明一个以状态名为键，以State接口为值的map
	stateByName map[string]State
	// 状态改变时的回调
	OnChange func(from, to State)
	// 当前状态
	curr State
}

// 添加一个状态到管理器中
func (sm *StateManager) Add(s State) {
	// 获取状态的名称
	name := StateName(s)
	// 将s转换为能设置名字的接口，然后调用该接口
	// 将s（State接口）通过类型断言转换为带有set Name()方法(name string)的接口。
	// 接着调用这个接口的set Name()方法设置状态的名称。使用该方法可以快速调用一个接口实现的其他方法
	s.(interface {
		setName(name string)
	}).setName(name)
	// 根据状态名获取已经添加的状态，检查该状态是否存在
	if sm.Get(name) != nil {
		panic("duplicate state:" + name)
	}
	// 根据名字保存到map中
	sm.stateByName[name] = s
}

// 根据名字获取指定状态
func (sm *StateManager) Get(name string) State {
	if v, ok := sm.stateByName[name]; ok {
		return v
	}
	return nil
}

// 初始化状态管理器
func NewStateManager() *StateManager {
	return &StateManager{
		stateByName: make(map[string]State),
	}
}

/**
 * @description: 在状态间转移
 */
// 状态没有找到的错误
var ErrStateNotFound = errors.New("state not found")

// 禁止在同状态间转移
var ErrForbidSameStateTransit = errors.New("forbid same state transit")

// 不能转移到指定状态
var ErrCannotTransitToState = errors.New("cannot transit to state")

// 获取当前的状态
func (sm *StateManager) CurrState() State {
	return sm.curr
}

// 当前状态能否转移到目标状态
func (sm *StateManager) CanCurrTransitTo(name string) bool {
	if sm.curr == nil {
		return true
	}
	// 相同的状态不用转换
	if sm.curr.Name() == name && !sm.curr.EnableSameTransit() {
		return false
	}
	// 使用当前状态，检查能否转移到指定名字的状态
	return sm.curr.CanTransitTo(name)
}

// 转移到指定状态
func (sm *StateManager) Transit(name string) error {
	// 获取目标状态
	next := sm.Get(name)
	// 目标不存在
	if next == nil {
		return ErrStateNotFound
	}
	// 记录转移前的状态
	pre := sm.curr
	// 当前有状态
	if sm.curr != nil {
		// 相同的状态不用转换
		if sm.curr.Name() == name && !sm.curr.EnableSameTransit() {
			return ErrForbidSameStateTransit
		}
		// 不能转移到目标状态
		if !sm.curr.CanTransitTo(name) {
			return ErrCannotTransitToState
		}
		// 结束当前状态
		sm.curr.OnEnd()
	}
	// 将当前状态切换为要转移到的目标状态
	sm.curr = next
	// 调用新状态的开始
	sm.curr.OnBegin()
	// 通知回调
	if sm.OnChange != nil {
		sm.OnChange(pre, sm.curr)
	}
	return nil
}

/**
 * @description: 自定义状态实现状态接口
 */

// 闲置状态
type IdleState struct {
	StateInfo // 使用State Info实现基础接口
}

// 重新实现状态开始
func (i *IdleState) OnBegin() {
	fmt.Println("Idle State begin")
}

// 重新实现状态结束
func (i *IdleState) OnEnd() {
	fmt.Println("Idle State end")
}

// 移动状态
type MoveState struct {
	StateInfo
}

func (m *MoveState) OnBegin() {
	fmt.Println("Move State begin")
}

// 允许移动状态互相转换
func (m *MoveState) EnableSameTransit() bool {
	return true
}

// 跳跃状态
type JumpState struct {
	StateInfo
}

func (j *JumpState) OnBegin() {
	fmt.Println("Jump State begin")
} // 跳跃状态不能转移到移动状态
func (j *JumpState) CanTransitTo(name string) bool {
	return name != "Move State"
}

func testInterface6() {
	// 实例化一个状态管理器
	sm := NewStateManager()
	// 响应状态转移的通知
	sm.OnChange = func(from, to State) {
		// 打印状态转移的流向
		fmt.Printf("%s ---> %s\n\n", StateName(from), StateName(to))
	}
	// 添加3个状态
	sm.Add(new(IdleState))
	sm.Add(new(MoveState))
	sm.Add(new(JumpState))
	// 在不同状态间转移
	transitAndReport(sm, "IdleState")
	transitAndReport(sm, "MoveState")
	transitAndReport(sm, "MoveState")
	transitAndReport(sm, "JumpState")
	transitAndReport(sm, "JumpState")
	transitAndReport(sm, "IdleState")
}

// 封装转移状态和输出日志
func transitAndReport(sm *StateManager, target string) {
	if err := sm.Transit(target); err != nil {
		fmt.Printf("FAILED! %s --> %s, %s\n\n", sm.CurrState().Name(), target, err.Error())
	}
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

//<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

func printType(v interface{}) {
	switch v.(type) {
	case int:
		fmt.Println(v, "is int")
	case string:
		fmt.Println(v, "is string")
	case bool:
		fmt.Println(v, "is bool")
	}
}
func testInterface5() {
	printType(1024)
	printType("pig")
	printType(true)
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
// 定义飞行动物接口
type Flyer interface {
	Fly()
}

// 定义行走动物接口
type Walker interface {
	Walk()
}

// 定义鸟类
type bird struct{}

// 实现飞行动物接口
func (b *bird) Fly() {
	fmt.Println("bird: fly")
}

// 为鸟添加Walk()方法，实现行走动物接口
func (b *bird) Walk() {
	fmt.Println("bird: walk")
}

// 定义猪
type pig struct{}

// 为猪添加Walk()方法，实现行走动物接口
func (p *pig) Walk() {
	fmt.Println("pig: walk")
}
func testInterface3() {
	// 创建动物的名字到实例的映射
	animals := map[string]interface{}{
		"bird": new(bird),
		"pig":  new(pig),
	}
	// 遍历映射
	for name, obj := range animals {
		// 使用类型断言获得f，类型为Flyer及is Flyer的断言成功的判定
		f, isFlyer := obj.(Flyer)
		// 判断对象是否为行走动物
		w, isWalker := obj.(Walker)
		fmt.Printf("name: %s is Flyer: %v is Walker: %v\n", name, isFlyer, isWalker)
		// 如果是飞行动物则调用飞行动物接口
		if isFlyer {
			f.Fly()
		}
		// 如果是行走动物则调用行走动物接口
		if isWalker {
			w.Walk()
		}
	}
}

func testInterface4() {
	p1 := new(pig)

	var a Walker = p1
	p2 := a.(*pig)

	fmt.Printf("p1=%p p2=%p", p1, p2)
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

//<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
/**
 * @description: 日志对外接口
 */
// 声明日志写入器接口
type LogWriter interface {
	Write(data interface{}) error
}

// 日志器
type Logger struct {
	// 这个日志器用到的日志写入器
	writerList []LogWriter
}

// 注册一个日志写入器
func (l *Logger) RegisterWriter(writer LogWriter) {
	l.writerList = append(l.writerList, writer)
}

// 将一个data类型的数据写入日志
func (l *Logger) Log(data interface{}) {
	// 遍历所有注册的写入器
	for _, writer := range l.writerList {
		// 将日志输出到每一个写入器中
		writer.Write(data)
	}
}

// 创建日志器的实例￼
func NewLogger() *Logger {
	return &Logger{}
}

// 声明文件写入器
/**
 * @description: 文件写入器
 * 文件写入器的功能是根据一个文件名创建日志文件（file Writer的Set File方法）。
 * 在有日志写入时，将日志写入文件中。
 */
// 声明文件写入器，在结构体中保存一个文件句柄，以方便每次写入时操作
type fileWriter struct {
	file *os.File
}

// 设置文件写入器写入的文件名
func (f *fileWriter) SetFile(filename string) (err error) {
	// 如果文件已经打开，关闭前一个文件
	// 考虑到SetFile()方法可以被多次调用（函数可重入性）
	// 假设之前已经调用过Set File()后再次调用，此时的f.file不为空，就需要关闭之前的文件，重新创建新的文件。
	if f.file != nil {
		f.file.Close()
	}
	// 创建一个文件并保存文件句柄
	f.file, err = os.Create(filename)
	// 如果创建的过程出现错误，则返回错误
	return err
}

// 实现LogWriter的Write()方法
func (f *fileWriter) Write(data interface{}) error {
	// 如果文件没有准备好，文件句柄为nil
	// 此时使用errors包的New()函数返回一个错误对象，包含一个字符串“file not created”
	if f.file == nil {
		// 日志文件没有准备好
		return errors.New("file not created")
	}
	// 将数据序列化为字符串
	// 使用fmt.Sprintf将data转换为字符串，这里使用的格式化参数是“%v”，意思是将data按其本来的值转换为字符串
	str := fmt.Sprintf("%v\n", data)
	// 通过f.file的Write()方法，将str字符串转换为[]byte字节数组，再写入到文件中。如果发生错误，则返回
	_, err := f.file.Write([]byte(str))
	return err
}

// 创建文件写入器实例
func newFileWriter() *fileWriter {
	return &fileWriter{}
}

/**命令行写入
 * @description:
 */
// 命令行写入器
type consoleWriter struct{}

// 实现LogWriter的Write()方法
func (f *consoleWriter) Write(data interface{}) error {
	// 将数据序列化为字符串
	str := fmt.Sprintf("%v\n", data)
	// 将数据以字节数组写入命令行中
	_, err := os.Stdout.Write([]byte(str))
	return err
}

// 创建命令行写入器实例
func newConsoleWriter() *consoleWriter {
	return &consoleWriter{}
}

/** 使用日志
 * @description:
 */

// 创建日志器
func testInterface2() {
	// 创建日志器
	l := NewLogger()
	// 创建命令行写入器
	cw := newConsoleWriter()
	// 注册命令行写入器到日志器中
	l.RegisterWriter(cw)
	// 创建文件写入器
	fw := newFileWriter()
	// 设置文件名
	if err := fw.SetFile("log.log"); err != nil {
		fmt.Println(err)
	}
	// 注册文件写入器到日志器中
	l.RegisterWriter(fw)

	// 写一个日志
	l.Log("hello")
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
// 定义一个数据写入器
type DataWriter interface {
	// interface{}类型的data，返回一个error结构表示可能发生的错误
	WriteData(data interface{}) error
}

// 定义文件结构，用于实现DataWriter
type file struct{}

// 实现DataWriter接口的WriteData()方法
func (d *file) WriteData(data interface{}) error {
	// 模拟写入数据
	fmt.Println("Write Data:", data)
	return nil
}

/** 接口的方法与实现接口的类型方法格式一致
 * @description:
 */
func testInterface1() {
	// 实例化file赋值给f，f的类型为*file
	f := new(file)
	// 声明一个DataWriter的接口
	var writer DataWriter
	// 将接口赋值f，也就是＊file类型
	writer = f
	// 使用DataWriter接口进行数据写入
	writer.WriteData("data")
}

//<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

// 车轮
type Wheel struct {
	Size int
}

// 车
type Car struct {
	Wheel
	// 引擎
	Engine struct {
		Power int    // 功率
		Type  string // 类型
	}
}

func testStruct3() {
	c := Car{
		// 初始化轮子（初始化结构体内嵌）
		Wheel: Wheel{
			Size: 18,
		},
		// 初始化引擎（初始化内嵌匿名结构体）
		Engine: struct {
			Power int
			Type  string
		}{
			Type:  "1.4T",
			Power: 143,
		},
	}
	fmt.Printf("%+v\n", c)
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
// 可飞行的
type Flying struct{}

func (f *Flying) Fly() {
	fmt.Println("can fly")
}

// 可行走的
type Walkable struct{}

func (f *Walkable) Walk() {
	fmt.Println("can calk")
}

// 人类
type Human struct {
	Walkable
	// 人类能行走
}

// 鸟类
type Bird struct {
	Walkable
	// 鸟类能行走
	Flying
	// 鸟类能飞行
}

func testStruct2() {
	// 实例化鸟类
	b := new(Bird)
	fmt.Println("Bird: ")
	b.Fly()
	b.Walk()
	// 实例化人类
	h := new(Human)
	fmt.Println("Human: ")
	h.Walk()
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
type BasicColor struct {
	R, G, B float32
}
type Color struct {
	// 结构体内嵌
	BasicColor
	Alpha float32
}

/**
 * @description: 声明结构体内嵌
 */
func testStruct1() {
	// 实例化一个完整颜色结构体
	var c Color
	c.R = 1
	c.G = 1
	c.B = 0
	c.Alpha = 1
	fmt.Printf("%+v", c)
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
// 实例化一个通过字符串映射函数切片的map
// 创建一个map实例，这个map通过事件名（string）关联回调列表（[]func(interface{}），
// 同一个事件名称可能存在多个事件回调，因此使用回调列表保存。回调的函数声明为func(interface{})
var eventByName = make(map[string][]func(interface{}))

/**事件注册
 * @description: 注册事件，提供事件名和回调函数
 */
func RegisterEvent(name string, callback func(interface{})) {
	// 通过名字查找事件列表
	list := eventByName[name]
	// 在列表切片中添加函数
	// 为同一个事件名称在已经注册的事件回调的列表中再添加一个回调函数
	list = append(list, callback)
	// 保存修改的事件列表切片
	eventByName[name] = list
}

/**事件调用
 * @description:调用事件
 */
func CallEvent(name string, param interface{}) {
	// 通过名字找到事件列表
	list := eventByName[name]
	// 遍历这个事件的所有回调
	for _, callback := range list {
		// 传入参数调用回调
		callback(param)
	}
}

/**使用事件系统
 * @description:
 */
// 声明角色的结构体
type Actor struct{}

// 为角色添加一个事件处理函数
// 拥有param参数，类型为interface{}，与事件系统的函数（func(interface{})）签名一致
func (a *Actor) OnEvent(param interface{}) {
	fmt.Println("actor event:", param)
}

// 全局事件
func GlobalEvent(param interface{}) {
	fmt.Println("global event:", param)
}
func testFuncMethod1() {
	// 实例化一个角色
	a := new(Actor)
	// 注册名为On Skill的回调
	RegisterEvent("On Skill", a.OnEvent) // 再次在OnSkill上注册全局事件
	RegisterEvent("On Skill", GlobalEvent)
	// 调用事件，所有注册的同名函数都会被调用
	CallEvent("On Skill", 100)
}

//<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
// 声明一个结构体
type class struct{}

// 给结构体添加Do()方法
func (c *class) Do(v int) {
	fmt.Println("call method do:", v)
}

// 普通函数的Do()方法
func funcDo(v int) {
	fmt.Println("call function do:", v)
}

/**
 * @description:方法和函数的统一调用
 */
func testFuncMethod() {
	// 声明一个函数回调
	var delegate func(int)
	// 创建结构体实例
	c := new(class)
	// 将回调设为c的Do方法
	delegate = c.Do
	// 调用
	delegate(100)
	// 将回调设为普通函数
	delegate = funcDo
	// 调用3
	delegate(100)
}

//<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
type Command struct {
	Name    string // 指令名称
	Var     *int   // 指令绑定的变量
	Comment string // 指令的解释
}

/**
 * @description: 取地址结构体实例化
 * @param {string} name
 * @param {*int} varref
 * @param {string} comment 描述
 * @return {*}
 */
func testStruct(name string, varref *int, comment string) *Command {
	return &Command{
		Name:    name,
		Var:     varref,
		Comment: comment}
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
// 崩溃时需要传递的上下文信息
type panicContext struct {
	function string // 所在函数
}

// 保护方式允许一个函数
func ProtectRun(entry func()) {
	// 使用defer将闭包延迟执行，当panic触发崩溃时，ProtectRun()函数将结束运行，此时defer后的闭包将会发生调用
	defer func() {
		// 发生宕机时，获取panic传递的上下文并打印
		// recover()获取到panic传入的参数
		err := recover()
		switch err.(type) {
		case runtime.Error: // 如果错误是有Runtime层抛出的运行时错误，如空指针访问、除数为0等情况，打印运行时错误
			fmt.Println("runtime error:", err)
		default: // 非运行时错误
			fmt.Println("error:", err)
		}
	}()
	entry()
}

/**
 * @description: 宕机处理
 * @param {*}
 * @return {*}
 */
func testPanic1() {
	fmt.Println("运行前")
	// 允许一段手动触发的错误
	ProtectRun(func() {
		fmt.Println("手动宕机前")
		// 使用panic传递上下文
		// 使用panic手动触发一个错误，并将一个结构体附带信息传递过去，此时，recover就会获取到这个结构体信息，并打印出来
		panic(&panicContext{
			"手动触发panic",
		})
		fmt.Println("手动宕机后")
	})
	// 故意造成空指针访问错误
	ProtectRun(func() {
		fmt.Println("赋值宕机前")
		var a *int
		// 模拟代码中空指针赋值造成的错误，此时会由Runtime层抛出错误，被ProtectRun()函数的recover()函数捕获到
		*a = 1
		fmt.Println("赋值宕机后")
	})
	fmt.Println("运行后")
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
func testPanic() {
	defer fmt.Println("💣 宕机后要做的事情1 ")
	defer fmt.Println("❌ 宕机后要做的事情2 ")

	panic("宕机")
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
// 声明一个解析错误
type ParseError struct {
	Filename string // 文件名
	Line     int    // 行号
}

// 实现error接口，返回错误描述
func (e *ParseError) Error() string {
	return fmt.Sprintf("%s:%d", e.Filename, e.Line)
}

/**
 * @description: 自定义Error
 * @param {*}
 * @return {*}
 */
func testError2() {
	var e error
	// 创建一个错误实例，包含文件名和行号
	e = &ParseError{"main.go", 1}

	// 通过error接口查看错误描述
	fmt.Println(e.Error())

	// 根据错误接口的具体类型，获取详细的错误信息
	switch detail := e.(type) {
	case *ParseError: // 这是一个解析错误
		fmt.Printf("Filename: %s Line: %d\n", detail.Filename, detail.Line)
	default: // 其他类型的错误
		fmt.Println("other error")
	}
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

/**
 * @description:除法错误测试
 * @param {*}
 * @return {*}
 */
// 定义除数为0的错误
var errDivisionByZero = errors.New("division by zero")

func testError1(dividend, divisor int) (int, error) {
	fmt.Printf("<=============== 🍎 🍎 🍎 ===============> \n\n")

	// 判断除数为0的情况并返回
	if divisor == 0 {

		return 0, errDivisionByZero
	}
	// 正常计算，打印空错误
	return dividend / divisor, nil

}

// <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
// 错误字符串
type errorString struct {
	s string
}

// 返回发生何种错误
// 实现error接口的Error()方法，该方法返回成员中的错误描述
func (e *errorString) Error() string {
	return e.s
}

/**
 * @description: 错误
 * @param {*}
 * @return {*}
 */
func testError(text string) {
	fmt.Printf("<=============== 🍎 🍎 🍎 ===============> \n\n")
	fmt.Print(&errorString{text})
}

//>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>

/**延迟语法
 * @description:
 * @param {*}
 * @return {*}
 */
func testDef() {
	fmt.Printf("<=============== 🍎 🍎 🍎 ===============> \n\n")

	filename := "/Users/harleyhuang/Documents/GitHub/Go/GoDemo/main.go"

	f, err := os.Open(filename)
	if err != nil {
		return
	}
	// 延迟调用Close，此时Close不会被调用
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		// defer机制触发，调用Close关闭文件
		return
	}
	size := info.Size()
	// defer机制触发，调用Close关闭文件
	fmt.Println("文件size：", size)
}

/**
 * @description: 可变参数
 * @param {*}
 * @return {*}
 */
func testVariableParameters(slist ...string) {
	fmt.Printf("<=============== 🍎 🍎 🍎 ===============> \n\n")

	// 定义一个字节缓冲，快速地连接字符串
	var b bytes.Buffer
	// 遍历可变参数列表slist，类型为[]string
	for _, s := range slist {
		// 将遍历出的字符串连续写入字节数组
		b.WriteString(s)
	}
	// 将连接好的字节数组转换为字符串并输出
	fmt.Printf(b.String())

}

/**
 * @description: 闭包的记忆效应
 * @param {*}
 * @return {*}
 */
func testClosure1_1() {
	fmt.Printf("<=============== 🍎 🍎 🍎 ===============> \n\n")

	// 创建一个累加器，初始值为1，
	// 返回的accumulator是类型为func() int的函数变量。
	accumulator := testClosure1(1)
	// 调用accumulator()时，开始执行func() int{}匿名函数逻辑，直到返回类加值
	fmt.Println(accumulator())
	fmt.Println(accumulator())
	// 打印累加器的函数地址
	fmt.Printf("%p\n", accumulator)
	// 创建一个累加器，初始值为
	accumulator2 := testClosure1(10)
	// 累加1并打印
	fmt.Println(accumulator2())
	// 打印累加器的函数地址
	fmt.Printf("%p\n", accumulator2)

}

/**
 * @description: 累加器生成函数，这个函数输出一个初始值，调用时返回一个为初始值创建的闭包函数
 * @param {*}
 * @return {*}
 */
func testClosure1(value int) func() int {

	// 返回一个闭包函数，每次返回会创建一个新的函数实例
	return func() int {
		// 对引用的testClosure1参数变量进行累加，
		// 注意value不是要返回的匿名函数定义的，但是被这个匿名函数引用，所以形成闭包。
		value++
		// 返回一个累加值
		return value
	}
}

// 函数定义为类型
type FuncCaller func(interface{})

// 实现Invoker的Call
func (f FuncCaller) Call(p interface{}) {
	// 调用f()函数本体
	f(p)
}

/**
 * @description: 函数实现接口
 * @param {*}
 * @return {*}
 */
func testFuncImplInterface1() {
	fmt.Printf("<=============== 🍎 🍎 🍎 ===============> \n\n")

	// 声明接口变量
	var invoker Invoker
	// 将匿名函数转为Func Caller类型，再赋值给接口
	invoker = FuncCaller(func(v interface{}) {
		fmt.Println("from function", v)
	})
	// 使用接口调用Func Caller.Call，内部会调用函数本体
	invoker.Call("🍓 函数接口 hello")
}

// 调用器接口
type Invoker interface {
	// 需要实现一个Call()方法
	Call(interface{})
}

type Struct struct{}

func (s *Struct) Call(p interface{}) {
	fmt.Println("from struct", p)
}

/**
 * @description: 结构体实现接口
 * @param {*}
 * @return {*}
 */
func testFuncImplInterface() {
	fmt.Printf("<=============== 🍎 🍎 🍎 ===============> \n\n")

	// 声明接口变量
	var invoker Invoker
	// 实例化结构体
	s := new(Struct)
	// 将实例化的结构体赋值到接口
	invoker = s
	// 使用接口调用实例化结构体的方法Struct.Call
	invoker.Call("🍎 hello 函数实现接口")
}

/**
 * @description: 匿名函数封装
 * @param {*}
 * @return {*}
 */
func testAnnoymousFunction1() {

	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	// 定义命令行skillParam，从命令行输入—skill可以将空格后的字符串传入skill Param指针变量
	var skillParam = flag.String("skill", "", "skill to perform")

	// 解析命令行参数，解析完成后，skillParam指针变量将指向命令行传入的值
	flag.Parse()

	// 定义一个从字符串映射到func()的map，然后填充这个map
	var skill = map[string]func(){
		"fire": func() {
			fmt.Println("chicken fire")
		},
		"run": func() {
			fmt.Println("soldier run")
		},
		"fly": func() {
			fmt.Println("angel fly")
		}}

	// skillParam是一个*string类型的指针变量，使用*skill Param获取到命令行传过来的值，并在map中查找对应命令行参数指定的字符串的函数
	if f, ok := skill[*skillParam]; ok {
		f()
	} else {
		fmt.Println("skill not found")
	}
}

// 遍历切片的每个元素，通过给定函数进行元素访问
func visit(list []int, f func(int)) {
	for _, v := range list {
		f(v)
	}
}

/**
 * @description: 匿名函数
 * @param {*}
 * @return {*}
 */
func testAnonymousFunction() {
	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	// 使用匿名函数打印切片内容18
	visit([]int{1, 2, 3, 4}, func(v int) {
		fmt.Println(v)
	})

	func(data int) {
		fmt.Println("hello", data)
	}(100)

	// 将匿名函数体保存到f()
	f := func(data int) {
		fmt.Println("hello", data)
	}
	// 使用f()调用
	f(100)

}

// 用于测试值传递效果的结构体
type Data struct {
	// 测试切片在参数传递中的效果
	complax []int

	instance InnerData
	// 实例分配的inner Data
	ptr *InnerData
	// 将ptr声明为Inner Data的指针类型
}

// 代表各种结构体字段
type InnerData struct {
	a int
}

func passByValue(inFunc Data) Data {
	// 输出参数的成员情况
	// 使用格式化的“%+v”动词输出in变量的详细结构，以便观察Data结构在传递前后的内部数值的变化情况
	fmt.Printf("in Func value: %+v\n", inFunc)
	// 打印inFunc的指针，在计算机中，拥有相同地址且类型相同的变量，表示的是同一块内存区域
	fmt.Printf("in Func ptr: %p\n", &inFunc)
	return inFunc
}

/**
 * @description: 值传递的测试函数
 * @param {*}
 * @return {*}
 */
func paramTranslate() {
	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	in := Data{
		//切片
		complax: []int{1, 2, 3},
		// 结构体
		instance: InnerData{
			5,
		},
		// 指针
		ptr: &InnerData{1},
	}
	// 输入结构的成员情况
	fmt.Printf("in value: %+v\n", in)
	// 输入结构的指针地址
	fmt.Printf("in ptr: %p\n", &in)
	// 传入结构体，返回同类型的结构体
	out := passByValue(in)
	// 输出结构的成员情况
	fmt.Printf("out value: %+v\n", out)
	// 输出结构的指针地址
	fmt.Printf("out ptr: %p\n", &out)
}

// 列表删除
func list_delete() {
	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	l := list.New()

	// 尾部添加
	l.PushBack("canon")
	// 头部添加
	l.PushFront(67)
	// 尾部添加后保存元素句柄
	element := l.PushBack("fist")
	// 在fist之后添加high
	l.InsertAfter("high", element)
	// 在fist之前添加noon
	l.InsertBefore("noon", element)
	// 使用
	l.Remove(element)

	for i := l.Front(); i != nil; i = i.Next() {
		fmt.Println(i.Value)
	}
}

// 九九乘法表：
func multiplication_table() {

	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	// 遍历，决定处理第几行
	for y := 1; y <= 9; y++ {
		// 遍历，决定这一行有多少列
		for x := 1; x <= y; x++ {
			fmt.Printf("%d＊%d=%d ", x, y, x*y)
		}
		// 手动生成回车
		fmt.Println()
	}
}

// 切片
func section_test() {
	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	var a = [4]int{10, 20, 30, 40}

	fmt.Println(a, "\n", a[1:3])
}

// 初始化数组
func init_array() {
	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	var member = [...]string{"曹可馨", "是个", "放屁精🐳", "曹智宸", "是个", "调皮鬼😝", "李亚", "是个", "小螃蟹🦀️"}

	for k, v := range member {
		fmt.Println(k, v)

	}

	scene := make(map[string]int)
	scene["route"] = 66
	fmt.Println(scene["route"])
	v := scene["route2"]

	fmt.Println(v)

	m := map[string]string{
		"W": "forward",
		"A": "left",
		"D": "right",
		"S": "backward",
	}

	for k, v := range m {
		fmt.Println(k, v)
	}

}

// 使用指针变量获取命令行的输入信息
func point_flag() {
	// 定义命令行参数
	/*
	* 3个参数分别如下：
	* 参数名称：在给应用输入参数时，使用这个名称
	* 参数值的默认值：与flag所使用的函数创建变量类型对应，String对应字符串、Int对应整型、Bool对应布尔型等
	* 参数说明：使用-help时，会出现在说明中
	 */
	var mode = flag.String("mode", "🍊 🍊", "process mode")

	// 解析命令行参数
	flag.Parse()

	fmt.Println(*mode)
}

// 函数的交换
func chargeValue() {
	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	// 准备两个变量，赋值1和2
	x, y := 1, 2
	// 交换变量值
	swap(&x, &y)

	fmt.Println(x, y)

}

func swap(a, b *int) {
	// 取a指针的值，赋给临时变量
	t := *a

	// 取b指针的值，赋给a指针指向的变量
	// 注意，此时“*a”的意思不是取a指针的值，而是“a指向的变量”
	*a = *b
	// 将a指针的值赋给b指针指向的变量
	*b = t
}

// 从指针获取指针指向的值
func pointTest1() {
	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	var house string = "🏠 房屋 366——26-404"

	ptr := &house

	fmt.Printf("ptr type: %T\n", ptr)
	fmt.Printf("address: %p\n", ptr)

	value := *ptr

	fmt.Printf("value type: %T\n", value)
	fmt.Printf("value: %s\n", value)
}

func pointTest0() {
	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	var cat int = 1
	var str string = "banana"

	fmt.Printf("%p %p ", &cat, &str)
}

// 短变量声明并初始化
func lowVar() {
	fmt.Printf("\n\n <=============== 🍎 🍎 🍎 ===============> \n\n")

	// Go语言的推导声明写法，编译器会自动根据右值类型推断出左值的对应类型
	// 注意：由于使用了“:=”，而不是赋值的“=”，因此推导声明写法的左值变量必须是没有定义过的变量。若定义过，将会发生编译错误。
	hp := 10

	// 注意：在多个短变量声明和赋值中，至少有一个新声明的变量出现在左值中，即便其他变量名可能是重复声明的，编译器也不会报错
	conn, err := net.Dial("tcp", "127.0.0.1: 8080")
	conn2, err := net.Dial("tcp", "127.0.0.1: 8080")

	fmt.Printf("hp: %d, conn: %s, err: %s, conn2: %s", hp, conn, err, conn2)

}

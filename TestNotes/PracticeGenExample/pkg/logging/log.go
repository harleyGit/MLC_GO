/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-02 16:04:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-02 20:45:12
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/pkg/logging/log.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

type Level int
var (
	F *os.File

	DefaultPrefix = ""
	DefaultCallerDepth = 2

	logger *log.Logger
	logPrefix = ""
	// 日志切片等级
	levelFlags = []string{"🕷️DEBUG", "🍒INFO", "‼️WARN", "❌ERROR", "🚫FATAL"}
)

const (
	DEBUG Level = iota
	INFO
	WARNING
	ERROR
	FATAL
)

func init() {
	filePath := getLogFileFullPath()
	F = openLogFile(filePath)

	// 创建一个新的日志记录器。out定义要写入日志数据的IO句柄。prefix定义每个生成的日志行的开头。flag定义了日志记录属性
	logger = log.New(F, DefaultPrefix, log.LstdFlags)
}

func Debug(v ...interface{}) {
	setPrefix(DEBUG)
	logger.Println(v)
}

func Info(v ...interface{}) {
	setPrefix(INFO)
	logger.Println(v)
}

func Warn(v ...interface{}) {
	setPrefix(WARNING)
	logger.Println(v)
}

func Error(v ...interface{}) {
	setPrefix(ERROR)
	logger.Println(v)
}

func Fatal(v ...interface{}) {
	setPrefix(FATAL)
	logger.Fatalln(v)
}



func setPrefix(level Level) {
	/* 
	Caller函数
	参数：
		skip：跳过栈帧的层级
			0 → 当前函数（即 runtime.Caller() 本身）
			1 → 调用 runtime.Caller() 的函数
			2 → 调用 runtime.Caller() 的函数的上层
			N → 继续向上追溯（通常用于日志、错误追踪） 
	返回值：
		pc（程序计数器，一般用不到，所以用 _ 忽略）
		file（调用者所在文件的路径）
		line（调用者所在的代码行号）
		ok（是否成功获取）
	*/
	_, file, line, ok := runtime.Caller(DefaultCallerDepth)
	if ok {
		logPrefix = fmt.Sprintf("[%s][%s:%d]", levelFlags[level], filepath.Base(file), line)
	} else {
		logPrefix = fmt.Sprintf("[%s]", levelFlags[level])
	}
	//  设置日志前缀
	logger.SetPrefix(logPrefix)
}


/*  func Caller(skip int) (pc uintptr, file string, line int, ok bool) {}举例详细用法
package main

import (
	"fmt"
	"runtime"
)

func logCallerInfo(skip int) {
	_, file, line, ok := runtime.Caller(skip)
	if ok {
		fmt.Printf("skip=%d → 文件: %s, 行号: %d\n", skip, file, line)
	} else {
		fmt.Printf("skip=%d → 无法获取调用者信息\n", skip)
	}
}

func testFunc() {
	logCallerInfo(0) // 获取 logCallerInfo() 本身的信息
	logCallerInfo(1) // 获取 testFunc() 的信息
	logCallerInfo(2) // 获取 main() 的信息
}

func main() {
	testFunc()
}



运行结果：
skip=0 → 文件: /Users/admin/main.go, 行号: 10
skip=1 → 文件: /Users/admin/main.go, 行号: 17
skip=2 → 文件: /Users/admin/main.go, 行号: 22

解释：
	skip=0 → 获取 logCallerInfo() 自己的文件和行号
	skip=1 → 获取 testFunc() 的文件和行号
	skip=2 → 获取 main() 的文件和行号
*/
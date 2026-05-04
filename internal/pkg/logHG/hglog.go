/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-19 16:18:09
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-27 21:00:57
 * @FilePath: /MLC_GO/pkg/hglog/hglog.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package logHG

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
)

type Level int

var (
	F *os.File

	DefaultPrefix      = ""
	DefaultCallerDepth = 2

	logger    *log.Logger
	logPrefix = ""
	// 日志切片等级
	levelFlags = []string{"🔥DEBUG", "🍒INFO", "‼️WARN", "❌ERROR", "🚫FATAL"}
)

const (
	DEBUG Level = iota
	INFO
	WARNING
	ERROR
	FATAL
)

func Setup() {
	// filePath := getLogFilePath()
	// fileName := getLogFileName()
	// F, err := openLogFile(fileName, filePath)
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	// 创建一个新的日志记录器。out定义要写入日志数据的IO句柄。prefix定义每个生成的日志行的开头。flag定义了日志记录属性
	logger = log.New(F, DefaultPrefix, log.LstdFlags)
}

// getCallerInfo 获取调用者的文件名和函数名
// skip=1 获取直接调用者，skip=2 获取调用者的调用者，依此类推
func getCallerInfo(skip int) string {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "???:0"
	}

	// 只保留文件名，去掉路径
	parts := strings.Split(file, "/")
	fileName := parts[len(parts)-1]

	// 获取函数名
	funcName := runtime.FuncForPC(pc).Name()
	// 只保留函数名，去掉包路径
	funcParts := strings.Split(funcName, ".")
	funcName = funcParts[len(funcParts)-1]

	return fmt.Sprintf("%s:%d %s", fileName, line, funcName)
}

func DebugInfo(v ...interface{}) {
	// setPrefix(DEBUG)
	caller := getCallerInfo(2)
	log.Printf("🔥 [%s] %v\n", caller, fmt.Sprint(v...))
}

// 比如： DebugFInfo("This is value: %v, and another: %d", "test", 42)
func DebugFInfo(format string, v ...interface{}) {
	// setPrefix(DEBUG)
	caller := getCallerInfo(2)
	log.Printf("🔥 [%s] "+format, append([]interface{}{caller}, v...)...)
}

func ErrInfo(v ...interface{}) {
	// setPrefix(ERROR)
	caller := getCallerInfo(2)
	log.Printf("❌ [%s] %v\n", caller, fmt.Sprint(v...))
}

func ErrFInfo(format string, v ...interface{}) {
	// setPrefix(ERROR)
	caller := getCallerInfo(2)
	log.Printf("❌ [%s] "+format, append([]interface{}{caller}, v...)...)
}

func FatalFInfo(format string, v ...interface{}) {
	// setPrefix(ERROR)
	caller := getCallerInfo(2)
	log.Printf("💣 [%s] "+format, append([]interface{}{caller}, v...)...)
	os.Exit(1)
}

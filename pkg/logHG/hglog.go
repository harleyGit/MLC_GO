/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-19 16:18:09
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-10 20:00:11
 * @FilePath: /MLC_GO/pkg/hglog/hglog.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package logHG

import (
	"log"
	"os"
)

type Level int
var (
	F *os.File

	DefaultPrefix = ""
	DefaultCallerDepth = 2

	logger *log.Logger
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

func DebugInfo(v ...interface{}) {
	// setPrefix(DEBUG)
	log.Println("🔥",v)
}
func ErrInfo(v ...interface{}) {
	// setPrefix(ERROR)
	log.Println("❌",v)
}
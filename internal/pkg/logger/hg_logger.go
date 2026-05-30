/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-27 21:02:12
  - @LastEditors: GangHuang harleysor@qq.com
  - @LastEditTime: 2026-02-01 11:43:11
* @FilePath: /MLC_GO/TestNotes/GenPracticeExample/pkg/logs/hg_logger.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* // 日志优化进阶：https://chatgpt.com/s/t_697eb834d3b481919281a7e65ea3d970
* // 日志进阶2: https://chatgpt.com/s/t_697ec14ec62081918ff9797326dd8528
* // 日志进阶3: https://chatgpt.com/s/t_697ecbc41ccc8191a7078cf99a0799fd
*/
package HGLoggerPackage

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

var writer *HGRollingFileWriter

/* 🔥更专业：通过环境变量控制-上线 / Docker / K8s 都用这一套
logDir := os.Getenv("LOG_DIR")
if logDir == "" {
	logDir = "logs"
}

启动时：LOG_DIR=/var/log/mlc-go go run cmd/server/main.go
*/

/*
	方式3️⃣：✅ 方式 3：实时 grep（非常实用）：

tail -f server.log | grep 1769383162123456000-193812
- 	一边请求接口
-	一边实时看某个 tid 的行为

方式1️⃣：查看日志通过tid：grep tid值 logs/server.log
比如：grep "e9504165c7068391e535b08403ac2df2" logs/server.log

方式2️⃣若是运行在服务器：journalctl -u your-service【Linux系统服务器名】 | grep tid值
比如：journalctl -u your-service | grep TID=050a9089e268470e2d324f4070424e84
-	Linux
-	systemd 管理的服务
-	例如你部署在服务器上：
*/
func Init() {

	logPath := getLogPath()
	logFile := filepath.Join(logPath, "server.log")

	w, err := NewRollintFileWriter(
		logFile,
		100*1024*1024, //100MB
		5,             // 保留5个
	)
	if err != nil {
		panic(err)
	}
	writer = w

	// 	mw 的效果是：
	// 	 写一次
	// 	 同时写到：
	// 		终端
	// 		server.log（自动切割）
	mw := io.MultiWriter(os.Stdout, w)

	log.SetOutput(mw) //代码输出到文件中,标准 log 写文件
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

func getLogPath() string {

	// 确保 logs 目录存在, 这个路径是相对路径，它相对于【🍎你启动程序时的工作目录】
	// 比如： cd MLC_GO
	// go run cmd/server/main.go
	// 那么 MLC_GO/logs/server.go
	logDir := "logs"
	root, _ := os.Getwd() // 使用相对路径，仍然和启动目录有关，只是更显示
	logPath := filepath.Join(root, logDir)

	return logPath
}

func GetLogWriter() *HGRollingFileWriter {

	return writer
}

// 程序退出前
func CloseLogger() {

	writer.file.Close()
}

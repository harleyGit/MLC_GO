/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-27 21:02:12
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-27 21:28:52
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/pkg/logs/hg_logger.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGLoggerPackage

import (
	"log"
	"os"
	"path/filepath"
)

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
	// 确保 logs 目录存在, 这个路径是相对路径，它相对于【🍎你启动程序时的工作目录】
	// 比如： cd MLC_GO
	// go run cmd/server/main.go
	// 那么 MLC_GO/logs/server.go
	logDir := "logs"
	root, _ := os.Getwd() // 使用相对路径，仍然和启动目录有关，只是更显示
	logPath := filepath.Join(root, logDir)
	_ = os.Mkdir(logPath, 0755)

	logFile := filepath.Join(logPath, "server.log")
	file, err := os.OpenFile(
		logFile,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		log.Fatalf("❌ERROR open log failed: %v", err)
	}
	defer file.Close()
	log.SetOutput(file) //代码输出到文件中
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

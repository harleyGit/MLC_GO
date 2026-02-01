/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 10:27:20
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-01 11:42:06
 * @FilePath: /MLC_GO/internal/logger/hg_rolling_log.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGLoggerPackage

import (
	"MLC_GO/internal/pkg/logHG"
	"os"
	"path/filepath"
	"sync"
)

type HGRollingFileWriter struct {
	mu       sync.Mutex
	file     *os.File
	fileName string
	maxSize  int64 // bytes
	maxBack  int   // 保留多少个历史文件
	size     int64
}

func NewRollintFileWriter(fileName string,
	maxSize int64, maxBack int) (*HGRollingFileWriter, error) {

	dir := filepath.Dir(fileName)
	os.MkdirAll(dir, 0755)

	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logHG.FatalFInfo("❌ERROR open log failed: %v", err)
		return nil, err
	}
	/** HGRollingFileWriter 成员变量 file 会被：
	a. HTTP handler
	b. middleware
	c. goroutine
	d. 全局 logger
	e. 长期使用
	f. 生命周期 = 整个进程

	g. defer file.Close(),若是写在这个函数中，这个函数执行完后会被立刻执行，
		g.1 文件描述符被关闭
		g.2 后续日志写入 → panic / silent fail / EBADF,无法被写入
		g.3 defer Close() 只适用于“短生命周期资源”
		g.4 日志文件是“进程级资源”，不要 defer 关闭
	*/
	info, _ := file.Stat()

	return &HGRollingFileWriter{
		file:     file,
		fileName: fileName,
		maxSize:  maxSize,
		maxBack:  maxBack,
		size:     info.Size(),
	}, nil
}

func (w *HGRollingFileWriter) Write(p []byte) (int, error) {

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.size+int64(len(p)) > w.maxSize {
		w.rotateLog()
	}

	n, err := w.file.Write(p)
	w.size += int64(n)

	return n, err
}

func (w *HGRollingFileWriter) rotateLog() {

	w.file.Close()

	// server.log.4 -> server.log.5
	for i := w.maxBack - 1; i >= 1; i-- {
		old := w.fileName + "." + itoa(i)
		new := w.fileName + "." + itoa(i+1)
		os.Rename(old, new)
	}

	// server.log -> server.log.1
	os.Rename(w.fileName, w.fileName+".1")

	f, _ := os.OpenFile(w.fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	w.file = f
	w.size = 0
}

func itoa(i int) string {
	return string('0' + byte(i))
}

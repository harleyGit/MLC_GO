/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 10:27:20
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-01 10:56:41
 * @FilePath: /MLC_GO/internal/logger/hg_rolling_log.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGLoggerPackage

import (
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

	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	info, _ := f.Stat()

	return &HGRollingFileWriter{
		file:     f,
		fileName: fileName,
		maxSize:  maxSize,
		maxBack:  maxBack,
		size:     info.Size(),
	}, nil
}

func (w *HGRollingFileWriter) WriteLog(p []byte) (int, error) {

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

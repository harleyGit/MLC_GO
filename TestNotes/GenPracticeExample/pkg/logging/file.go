/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-02 16:04:38
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 19:46:30
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/pkg/logging/file.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
/*
 const (
    // Exactly one of O_RDONLY, O_WRONLY, or O_RDWR must be specified.
    O_RDONLY int = syscall.O_RDONLY // 以只读模式打开文件
    O_WRONLY int = syscall.O_WRONLY // 以只写模式打开文件
    O_RDWR   int = syscall.O_RDWR   // 以读写模式打开文件
    // The remaining values may be or'ed in to control behavior.
    O_APPEND int = syscall.O_APPEND // 在写入时将数据追加到文件中
    O_CREATE int = syscall.O_CREAT  // 如果不存在，则创建一个新文件
    O_EXCL   int = syscall.O_EXCL   // 使用O_CREATE时，文件必须不存在
    O_SYNC   int = syscall.O_SYNC   // 同步IO
    O_TRUNC  int = syscall.O_TRUNC  // 如果可以，打开时
 )
*/
package logging

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/file"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/setting"
	"fmt"
	"log"
	"os"
	"time"
)

/*
 var (
	// 日志路径
	 LogSavePath = "./TestNotes/GenPracticeExample/runtime/logs/"
	 LogSaveName = "log"
	 LogFileExt = "log"
	 TimeFormat = "20060102"
 )
*/
 func getLogFilePath() string {
	return fmt.Sprintf("%s%s", setting.AppSetting.RuntimeRootPath, setting.AppSetting.LogSavePath)
 }


func openLogFile(fileName, filePath string) (*os.File, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd err: %v", err)
	}

	src := dir + "/" + filePath
	perm := file.CheckPermission(src)
	if perm == true {
		return nil, fmt.Errorf("file.CheckPermission Permission denied src: %s", src)
	}

	err = file.IsNotExistMkDir(src)
	if err != nil {
		return nil, fmt.Errorf("file.IsNotExistMkDir src: %s, err: %v", src, err)
	}

	f, err := file.Open(src + fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("Fail to OpenFile :%v", err)
	}

	return f, nil
}
 
 func getLogFileName() string {
	return fmt.Sprintf("%s%s.%s",
		setting.AppSetting.LogSaveName,
		time.Now().Format(setting.AppSetting.TimeFormat),
		setting.AppSetting.LogFileExt,
	)
}

 func getLogFileFullPath() string {
	 prefixPath := getLogFilePath()
	 suffixPath := fmt.Sprintf("%s%s.%s", setting.AppSetting.LogSaveName, time.Now().Format(setting.AppSetting.TimeFormat), setting.AppSetting.LogFileExt)
 
	 return fmt.Sprintf("%s%s", prefixPath, suffixPath)
 }
 
 /* 
 检查文件是否存在：
	不存在 → 创建目录，并准备创建文件
	无权限 → 直接 log.Fatalf 退出程序
	以追加模式打开文件（如果文件不存在则创建）
	返回文件句柄 *os.File，用于后续日志写入
 */
 func openLogFileV1(filePath string) *os.File {
	 // os.Stat ：返回文件信息结构描述文件。如果出现错误，会返回*PathError
	 // os.Stat(filePath) 获取文件信息， 返回： 文件信息（os.FileInfo），错误信息（error）
	 _, err := os.Stat(filePath)
	 switch {
		 case os.IsNotExist(err): // 文件不存在；能够接受ErrNotExist、syscall的一些错误，它会返回一个布尔值，能够得知文件不存在或目录不存在
			 mkDir()
		 case os.IsPermission(err): // 文件无权限；能够接受ErrPermission、syscall的一些错误，它会返回一个布尔值，能够得知权限是否满足
		 log.Fatalf("❌ Permission: %v", err)
	 }
 
	 // os.OpenFile: 调用文件，支持传入文件名称、指定的模式调用文件、文件权限，返回的文件的方法可以用于 I/O。如果出现错误，则为*PathError。
	 /*
	 	os.OpenFile 打开文件，如果文件不存在，则创建：
			os.O_APPEND → 追加模式（写入时不会覆盖已有内容）
			os.O_CREATE → 文件不存在就创建
			os.O_WRONLY → 只写模式
			0644 → 文件权限（rw-r--r--）
				6 → 所有者 rw-（可读写）
				4 → 组 r--（可读）
				4 → 其他用户 r--（可读）
	 */
	 handle, err := os.OpenFile(filePath, os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0644)
	 if err != nil {
		log.Fatalf("❌ Fail to OpenFile: %v", err)
	 }
 
	 // 成功打开文件后，返回 *os.File 句柄，用于日志写入。
	 return handle
 }
 
 func mkDir() {
	 dir, _ := os.Getwd() // 获取当前工作目录（working directory），并存储在 dir 变量中。
	 // os.MkdirAll：创建对应的目录以及所需的子目录，若成功则返回nil，否则返回error
	 // os.ModePerm：const定义ModePerm FileMode = 0777
	 // os.MkdirAll(logDir, 0755) → 递归创建 logs/ 目录
	 err := os.MkdirAll(dir + "/" + getLogFilePath(), os.ModePerm)
	 if err != nil {
		 panic(err)
	 }
 }
 
 
 
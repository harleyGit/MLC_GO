/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-18 16:55:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-08-02 17:32:30
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/nsq_project_practice/nsq_practice_v1/nsq_practice_v1_v1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package nsq_practice_v1

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/pkg/logHG"
	"context"
	"crypto/md5"
	"flag"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Options struct {
	ID int64 `flag:"node-id" cfg:"id"` // 命令行参数名 --node-id，配置文件键名 id
	// LogLevel  lg.LogLevel `flag:"log-level"`             // 命令行参数名 --log-level，无配置文件键
	LogPrefix string `flag:"log-prefix"` // 命令行参数名 --log-prefix
	// Logger    Logger                                     // 无标签，不通过命令行或配置文件设置

	TCPAddress  string `flag:"tcp-address"`  // 命令行参数名 --tcp-address
	HTTPAddress string `flag:"http-address"` // 命令行参数名 --http-address
}

type NSQPracticeV1 struct {
}
// 协议
func (nsqPracticeV1 *NSQPracticeV1) ExecutePracticeNone() {}

func (nsqPracticeV1 *NSQPracticeV1) NSQPracticeV1() {
	nsqPractice_V1_v1()
}

/* 上下文取消 */
func (this *NSQPracticeV1) NSQCancelContext() {

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())

	// 启动一个 goroutine 监听这个context
	go func ()  {
		select {
		case <- ctx.Done():
			logHG.DebugFInfo("✅ 收到取消信号:", ctx.Err())
		}
	}()

	// 主程序等待 2 秒再取消
	time.Sleep(2 * time.Second)
	logHG.DebugInfo("⛔ 手动取消任务")
	cancel()

	//给 goroutine 一点时间打印
	time.Sleep(1 *time.Second)

}

func (this *NSQPracticeV1) NSQCustomLog() {
	// 创建一个带前缀和微秒时间戳的 Logger
	logger := log.New(os.Stderr, "[MyApp] ", log.Ldate|log.Ltime|log.Lmicroseconds)

	// 打印几条日志看看效果
	logger.Println("启动服务中...")
	logger.Println("连接数据库成功")
	logger.Println("监听端口 8080")

	f, _ := os.Create("/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO/app.log")
	logger01 := log.New(f, "[MyApp] ", log.Ldate|log.Ltime|log.Lmicroseconds)
	logger01.Println("写入日志文件---------实打实的发送哈")
}

// 路径拼接
func (this *NSQPracticeV1) NSQFilePathPT() {

	cwd, err := os.Getwd()
	if err != nil {
		logHG.DebugInfo("获取路径错误")
		return
	}
	configPath := filepath.Join(cwd, "TestNotes/unfamiliar_grammar_practice/nsq_project_practice", "NSQ_README.md")
	logHG.DebugFInfo("路径为：%+v", configPath)

}

/* 命令行参数解析 */
func (this *NSQPracticeV1) NSQPraCMDParse() {

	// 创健一个参数解析器
	flagSet := flag.NewFlagSet("HuangGang_CMD1009", flag.ExitOnError)

	//定义参数
	version := flagSet.Bool("version", false, "print version string")
	port := flagSet.Int("port", 8080, "set port")

	logHG.DebugInfo("当前命令行参数：", os.Args)

	// 正确跳过 os.Args[0]，只解析你想传的参数。
	// 解析传入的命令行参数
	flagSet.Parse(os.Args[1:])

	// 使用参数值
	if *version { //因为在lauch.json设置为false，所以不打印
		logHG.DebugInfo("版本显示：", *version, "MLC_GO version V1.0.0.0")
		//os.Exit(0)
	}

	logHG.DebugInfo("启动端口：", *port)
}

// 测试主机名的获取
func nsqPractice_V1_v1() {
	// 1. 获取当前操作系统的主机名（如 localhost 或服务器名）。: "GangHuangs-MacBook-Pro.local"
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err) // 若获取失败，直接终止程序
	}

	// 2. 使用 MD5 哈希算法初始化一个哈希对象
	// 生成 128 位（16字节） 的哈希值
	h := md5.New()

	// 3. 将主机名写入哈希对象（计算 MD5 哈希值）
	// 将主机名字符串转换为字节流，并写入 MD5 哈希对象
	io.WriteString(h, hostname)
	logHG.DebugInfo("--md5 h:", h.Sum(nil))

	// 4. 生成最终的 defaultID：
	//    - 先计算 MD5 哈希值的 CRC32 校验和
	//    - 然后对 1024 取模，得到 0~1023 的整数
	// h.Sum(nil)：获取 MD5 哈希值的字节切片（16字节）
	// crc32.ChecksumIEEE(...)：计算该字节切片的 CRC32 校验和，得到一个 32 位无符号整数（范围 0~4294967295）
	// % 1024：对 CRC32 结果取模，将其限制在 0~1023
	defaultID := int64(crc32.ChecksumIEEE(h.Sum(nil)) % 1024)

	options := &Options{
		ID: defaultID,
	}

	logging.DebugInfo("域名 hostname:", hostname, "md5 h:", h.Sum(nil),
		"defaultID", defaultID, "options 信息: ", options)
}



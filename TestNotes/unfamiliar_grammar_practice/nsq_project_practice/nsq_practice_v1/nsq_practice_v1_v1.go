package nsq_practice_v1

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"crypto/md5"
	"hash/crc32"
	"io"
	"log"
	"os"

)

type Options struct {
    ID        int64       `flag:"node-id" cfg:"id"`      // 命令行参数名 --node-id，配置文件键名 id
    // LogLevel  lg.LogLevel `flag:"log-level"`             // 命令行参数名 --log-level，无配置文件键
    LogPrefix string      `flag:"log-prefix"`            // 命令行参数名 --log-prefix
    // Logger    Logger                                     // 无标签，不通过命令行或配置文件设置

    TCPAddress  string `flag:"tcp-address"`             // 命令行参数名 --tcp-address
    HTTPAddress string `flag:"http-address"`            // 命令行参数名 --http-address
}

type NSQPracticeV1 struct {

}


func (nsqPracticeV1 *NSQPracticeV1) NSQPracticeV1() {
	nsqPractice_V1_v1()
}


// 测试主机名的获取
func nsqPractice_V1_v1(){
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

	// 4. 生成最终的 defaultID：
	//    - 先计算 MD5 哈希值的 CRC32 校验和
	//    - 然后对 1024 取模，得到 0~1023 的整数
	// h.Sum(nil)：获取 MD5 哈希值的字节切片（16字节）
	// crc32.ChecksumIEEE(...)：计算该字节切片的 CRC32 校验和，得到一个 32 位无符号整数（范围 0~4294967295）
	// % 1024：对 CRC32 结果取模，将其限制在 0~1023
	defaultID := int64(crc32.ChecksumIEEE(h.Sum(nil)) % 1024)

	options := &Options{
		ID:        defaultID,
	}

	logging.DebugInfo("options 信息: ", options)
}

// 协议
func (nsqPracticeV1 *NSQPracticeV1) ExecutePracticeNone() {}
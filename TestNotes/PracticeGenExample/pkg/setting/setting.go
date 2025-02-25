/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-23 20:50:34
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-02-25 12:08:04
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/pkg/setting/setting.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package setting

import (
	"log"
	"time"

	"gopkg.in/ini.v1"
)

var (
	/*
	Cfg *ini.File
		Cfg 是一个指向 ini.File 的指针，通常用于存储和操作 .ini 配置文件。
		ini.File 可能来自于 github.com/go-ini/ini 这个库，用于解析和读取 .ini 配置文件。
		这个变量通常会在程序启动时通过 Cfg, err = ini.Load("config.ini") 这样的方式初始化。
	*/
	Cfg *ini.File
	
	RunMode string

	HTTPPort int
	/*
	ReadTimeout 是一个 time.Duration 类型的变量，用于指定某个读取操作的超时时间。
	time.Duration 是 Go 中表示时间长度的类型，单位是纳秒（ns）。
	*/
	ReadTimeout time.Duration
	WriteTimeout time.Duration

	PageSize int
	JwtSecret string

)

func init() {
	var err error
	Cfg, err = ini.Load("./../../conf/app.ini")// 注意：地址是相对于setting.go的地址的
	if err != nil {
		log.Fatalf("Fail to parse 'conf/app.ini': %v", err)
	}

	LoadBase()
	LoadServer()
	LoadApp()
}

func LoadBase() {
	RunMode = Cfg.Section("").Key("RUN_MODE").MustString("debug")
}

func LoadServer() {
	sec, err := Cfg.GetSection("server")

	if err != nil {
		log.Fatalf("Fail to get section 'server' : %v", err)
	}

	//MustInt(8000) 是 Go 语言中 ini 配置库（通常是 github.com/go-ini/ini）的一个方法，用于读取 .ini 配置文件中的整数值，如果读取失败，则返回默认值 8000。
	HTTPPort = sec.Key("HTTP_PORT").MustInt(8000)
	ReadTimeout = time.Duration(sec.Key("READ_TIMEOUT").MustInt(60)) * time.Second
	WriteTimeout = time.Duration(sec.Key("WRITE_TIMEOUT").MustInt(60)) * time.Second
}

func LoadApp() {
	sec, err := Cfg.GetSection("app")
	if err != nil {
		log.Fatalf("Fail to get section 'app': %v", err)
	}
	JwtSecret = sec.Key("JWT_SECRET").MustString("!@)*#)!@U#@*!@!)")
	PageSize = sec.Key("PAGE_SISE").MustInt(10)
}
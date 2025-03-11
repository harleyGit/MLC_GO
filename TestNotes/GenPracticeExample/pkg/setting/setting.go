/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-23 20:50:34
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-11 19:39:29
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/pkg/setting/setting.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package setting

import (
	"log"
	"time"

	"gopkg.in/ini.v1"
)

type App struct {
	JwtSecret string	
	PageSize  int
	PrefixUrl string

	RuntimeRootPath string

	ImageSavePath string
	ImageMaxSize int
	ImageAllowExts []string

	ExportSavePath string
	QrCodeSavePath string
	FontSavePath string

	LogSavePath string
	LogSaveName string
	LogFileExt string
	TimeFormat string
}
/*
	这里的 AppSetting 仍然是一个全局变量，它是一个指向 App 结构体的指针。
	&App{} 是创建了一个新的 App 类型的实例，并返回它的地址。
	注意，App{} 是一个零值的 App 实例，也就是说：
		Name 字段的零值是空字符串 ""。
		Port 字段的零值是 0。

*/
var AppSetting = &App{}

type Server struct{
	RunMode string	//不区分大小写写法： `ini:"runmode"`，否则默认区分大小写
	HttpPort int
	ReadTimeout time.Duration
	WriteTimeout time.Duration
}
var ServerSetting = &Server{}


type Database struct {
	Type string
	User string
	Password string
	Host string
	Name string
	TablePrefix string
}
var DatabaseSetting = &Database{}

type Redis struct {
	Host string
	Password string
	MaxIdle int
	MaxActive int
	IdleTimeout time.Duration
}
var RedisSetting = &Redis{}

var cfg *ini.File
/*
var (
	// Cfg *ini.File
	// 		Cfg 是一个指向 ini.File 的指针，通常用于存储和操作 .ini 配置文件。
	// 		ini.File 可能来自于 github.com/go-ini/ini 这个库，用于解析和读取 .ini 配置文件。
	// 		这个变量通常会在程序启动时通过 Cfg, err = ini.Load("config.ini") 这样的方式初始化。
	Cfg *ini.File

	RunMode string

	HTTPPort int

	// ReadTimeout 是一个 time.Duration 类型的变量，用于指定某个读取操作的超时时间。
	// time.Duration 是 Go 中表示时间长度的类型，单位是纳秒（ns）。
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	PageSize  int
	JwtSecret string
)
*/

func Setup() { //MLC_GO/TestNotes/GenPracticeGenExample/conf/app.ini
	var err error
	// 注意：单元测试地址是相对于setting.go的地址的 ./../../conf/app.ini, 在main.go运行时地址为：./TestNotes/PracticeGenExample/conf/app.ini
	cfg, err = ini.Load("./TestNotes/GenPracticeExample/conf/app.ini") 
	if err != nil {
		log.Fatal("❌ setting.Setup, fail to parse 'conf/app.ini': %v", err)
	}

	// MapTo()将 app 部分的数据映射到 AppSetting 结构体中
	err = cfg.Section("app").MapTo(AppSetting)
	if err != nil {
		log.Fatal("❌ Cfg.MapTo AppSetting err: ", err)
	}

	AppSetting.ImageMaxSize = AppSetting.ImageMaxSize * 1024 * 1024

	err = cfg.Section("server").MapTo(ServerSetting)
	if err != nil {
		log.Fatal("❌ Cfg.MapTo ServerSetting err: %v", err)
	}

	ServerSetting.ReadTimeout = ServerSetting.ReadTimeout * time.Second
	ServerSetting.WriteTimeout = ServerSetting.WriteTimeout * time.Second

	err = cfg.Section("database").MapTo(DatabaseSetting)
	if err != nil {
		log.Fatal("❌ Cfg.MapTo DatabaseSetting err: %v", err)
	}
	
	// LoadBase()
	// LoadServer()
	// LoadApp()

	/*mapTo("app", AppSetting)
	mapTo("server", ServerSetting)
	mapTo("database", DatabaseSetting)
	mapTo("redis", RedisSetting)

	AppSetting.ImageMaxSize = AppSetting.ImageMaxSize *1024 * 1024
	ServerSetting.ReadTimeout = ServerSetting.ReadTimeout * time.Second
	ServerSetting.WriteTimeout = ServerSetting.WriteTimeout * time.Second
	RedisSetting.IdleTimeout = RedisSetting.IdleTimeout *time.Second
	*/
}



// v：是一个接口类型（interface{}），它允许传入任何类型的变量。通常这个参数会是一个结构体，用来接收从配置文件映射过来的数据。
func mapTo(section string, v interface{}) {
	 
	// Cfg.Section(section)：这是获取配置文件中的某个特定 section。Cfg 是某个配置管理对象，通常是从配置文件（如 INI、YAML 等）读取的配置。
	// .MapTo(v)：该方法将配置文件中该 section 的内容映射到 v 中。也就是说，它会根据 section 中的键值对填充 v 变量。如果 v 是一个结构体，MapTo 会将 section 中的配置值按字段名与结构体字段进行匹配，填充结构体字段的值
	err := cfg.Section(section).MapTo(v)
	if err != nil {
		log.Fatal("Cfg.MapTo %s err: %v", section, err)
	}
}

/* 设置优化后的代码
func LoadBase() {
	RunMode = Cfg.Section("").Key("RUN_MODE").MustString("debug")
}

func LoadServer() {
	sec, err := Cfg.GetSection("server")

	if err != nil {
		logging.Fatal("Fail to get section 'server' : %v", err)
	}

	//MustInt(8000) 是 Go 语言中 ini 配置库（通常是 github.com/go-ini/ini）的一个方法，用于读取 .ini 配置文件中的整数值，如果读取失败，则返回默认值 8000。
	HTTPPort = sec.Key("HTTP_PORT").MustInt(8000)
	ReadTimeout = time.Duration(sec.Key("READ_TIMEOUT").MustInt(60)) * time.Second
	WriteTimeout = time.Duration(sec.Key("WRITE_TIMEOUT").MustInt(60)) * time.Second
}

func LoadApp() {
	sec, err := Cfg.GetSection("app")
	if err != nil {
		logging.Fatal("❌ Fail to get section 'app': %v", err)
	}
	JwtSecret = sec.Key("JWT_SECRET").MustString("!@)*#)!@U#@*!@!)")
	PageSize = sec.Key("PAGE_SISE").MustInt(10)
}
*/
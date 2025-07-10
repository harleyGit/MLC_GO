/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-21 11:04:09
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-21 12:57:29
 * @FilePath: /MLC_GO/pkg/hg_setup_config/hg_setup_config_v1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package hg_setup_config

import (
	"MLC_GO/pkg/logHG"
	"flag"
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

const (
	ConfigEnv  = "GVA_CONFIG"
	ConfigFile = "./conf/mlc_app.yaml"
)

/*
mapstructure:"xxx"

	用于 viper 解析配置文件（YAML/JSON 等）到 Go 结构体时，进行字段匹配。
	作用： 指定 viper.Unmarshal() 解析时，如何映射配置文件的键值。

json:"xxx"

	用于 JSON 序列化和反序列化（json.Marshal() 和 json.Unmarshal()）。
	作用： 当 json.Marshal(config.GVA_CONFIG) 时，指定字段对应的 JSON key。

yaml:"xxx"

	用于 YAML 解析（yaml.Unmarshal() 和 yaml.Marshal()）。
	作用： 指定结构体字段在 YAML 格式中的映射
*/
type Server struct {
	// Zap    Zap    `mapstructure:"zap" json:"zap" yaml:"zap"`
	System System `mapstructure:"system" json:"system" yaml:"system"`
	// Mysql  Mysql  `mapstructure:"mysql" json:"mysql" yaml:"mysql"`
}

type System struct {
	Env    string `mapstructure:"env" json:"env" yaml:"env"`
	Addr   int    `mapstructure:"addr" json:"addr" yaml:"addr"`
	DbType string `mapstructure:"db-type" json:"dbType" yaml:"db-type"`
}

type HGSetupConfig struct{}

var (
	appConfigPath string = "./conf/mlc_app.json"
	appConfig     string
	otherConfig   *string

	GVA_CONFIG Server
)

/// 使用命令行参数: ./app -c /etc/myconfig.yaml

/// 使用环境变量: export CONFIG_PATH="/home/user/config.yaml"
/// 		./app

// / 默认使用本地文件: ./app
func (hgSetupConfig *HGSetupConfig) HGSetupConfig() {
	/// 1. 处理命令行参数
	// configCon 是存储配置文件路径的变量。
	// -c 是命令行参数，比如 ./app -c config.yaml。
	// 如果 命令行未传 -c 参数，configCon 默认为 ""。
	flag.StringVar(&appConfig, "c", "", "选择一个配置文件")
	// 解析命令行参数，获取 -c 传入的值
	flag.Parse()

	// 优先级: 命令行 > 环境变量 > 默认值
	if appConfig == "" { // 如果没有 -c 参数，就检查 os.Getenv(utils.ConfigEnv) 是否存在
		/// 2. 读取环境变量
		if configEnv := os.Getenv(ConfigEnv); configEnv == "" {
			// 如果环境变量为空 → 使用 ConfigFile（默认配置文件）
			appConfig = ConfigFile
			logHG.DebugInfo("使用本地配置文件,路径: ", ConfigFile)
		} else { // 如果环境变量不为空 → 使用环境变量指定的配置路径
			appConfig = configEnv
			logHG.DebugInfo("使用远程配置文件,路径: ", configEnv)
		}
	}

	/// 使用 viper 读取配置文件
	// 创建 viper 实例
	v := viper.New()
	// 指定要读取的配置文件
	v.SetConfigFile(appConfig)
	// 读取文件内容
	if err := v.ReadInConfig(); err != nil { // 如果 ReadInConfig() 失败（文件不存在或格式错误），就会触发 panic
		panic(fmt.Errorf("读取配置文件失败: %s \n", err))
	}
	// 让 viper 监听配置文件，如果文件内容变化，会自动更新 viper 读取的配置
	v.WatchConfig()

	v.OnConfigChange(func(e fsnotify.Event) {
		// 当配置文件发生变化时，打印 "配置文件已修改并更新"
		logHG.DebugInfo("配置文件已修改并更新: ", e.Name)
		if err := v.Unmarshal(&GVA_CONFIG); err != nil { // 重新解析配置，确保最新的内容被加载。
			logHG.ErrInfo("配置文件解析失败: ", err)
		}
	})

	/// 解析配置到结构体
	// 将 viper 读取的 YAML/JSON 配置解析到 GVA_CONFIG 结构体中，方便程序访问
	if err := v.Unmarshal(&GVA_CONFIG); err != nil {
		logHG.ErrInfo("===配置文件解析失败: ", err)
	}

	logHG.DebugInfo("配置文件解析成功: ", v)

}
func (hgSetupConfig *HGSetupConfig) ExecutePracticeNone() {
	logHG.DebugInfo("初始化配置文件")
}

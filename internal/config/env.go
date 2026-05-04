/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-12-22 16:49:51
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-04 11:19:49
 * @FilePath: /MLC_GO/config/server_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package ConfigPackage

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Env string

const (
	EnvDebug Env = "debug"
	EnvPre   Env = "pre"
	EnvProd  Env = "prod"
)

// 获取lauch.json中设置的环境变量 SERVER_ENV 的值，返回对应的 Env 类型
func GetEnv() Env {

	// VSCode 读取 launch.json，VSCode 读取 launch.json，工程启动后，dlv 创建子进程，注入 env 中的环境变量
	// Go 程序启动，进程的环境变量包含 SERVER_ENV=debug
	env := os.Getenv("SERVER_ENV")
	if env == "" {
		return EnvDebug // 默认环境为 debug

	}

	return Env(env)
}

/*
加载 YAML 配置文件（使用 Viper）
* @Description: 加载不同环境配置【业界主流】；
读取/config/config.debug.yaml路径下的内容，并解析成结构体
*/
func LoadConfig(env string) error {
	viper.SetConfigName(fmt.Sprintf("config.%s", env))
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	err := viper.ReadInConfig()

	return err
}

func IsDebug() bool {
	return GetEnv() == EnvDebug
}

func IsPre() bool {
	return GetEnv() == EnvPre
}

func IsProd() bool {
	return GetEnv() == EnvProd
}

func GetServerPort() string {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		return "8080" // 默认端口
	}
	return port
}

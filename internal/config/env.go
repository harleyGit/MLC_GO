/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-12-22 16:49:51
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-12 21:22:17
 * @FilePath: /MLC_GO/config/server_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Env string

const (
	EnvDebug Env = "debug"
	EnvPre Env = "pre"
	EnvProd Env = "prod"
)

func GetEnv() Env {
	
	env := os.Getenv("SERVER_ENV")
	if env == "" {
		return EnvDebug // 默认环境为 debug
	
	}

	return Env(env)
}

/* 加载不同环境配置【业界主流】 */
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




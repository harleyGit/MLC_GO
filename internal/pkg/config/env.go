/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-12-22 16:49:51
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-25 14:28:36
 * @FilePath: /MLC_GO/config/server_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package ConfigPackage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Env string

const (
	EnvDebug Env = "debug"
	EnvPre   Env = "pre"
	EnvProd  Env = "prod"

	hgDefaultConfigDir = "./config"
	hgLocalEnvFileName = "MLC.local.env"
	hgLoadedEnvKey     = "runtime.loaded_env"

	// 环境变量控制，从终端中输入，避免代码泄漏。输入环境变量，如：SERVER_ENV=debug
	hgConfigDirEnv          = "MLC_CONFIG_DIR"
	hgDebugMySQLPasswordEnv = "MLC_DEBUG_MYSQL_PASSWORD"
)

// hgLoadLocalRuntimeEnv 加载不提交到 Git 的本机覆盖文件。
// godotenv.Load 不覆盖进程已有环境变量，因此优先级为：外部注入 > MLC.local.env > 共享配置。
func hgLoadLocalRuntimeEnv(configDir string) error {
	localEnvFile := filepath.Join(configDir, hgLocalEnvFileName)
	if err := godotenv.Load(localEnvFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("加载本机运行环境文件 %q 失败: %w", localEnvFile, err)
	}
	return nil
}

// InitRuntimeEnv 依次加载可选的 MLC.local.env 和共享的 MLC.env。
// godotenv.Load 不覆盖进程已有变量，因此优先级为外部注入、本机覆盖、共享默认值。
func InitRuntimeEnv() error {
	configDir := os.Getenv(hgConfigDirEnv)
	if configDir == "" {
		configDir = hgDefaultConfigDir
	}
	if err := hgLoadLocalRuntimeEnv(configDir); err != nil {
		return err
	}

	envFile := filepath.Join(configDir, "MLC.env")
	// godotenv.Load: 读取指定的 .env 文件，将里面的键值对加载到当前进程的环境变量中，之后可以通过 os.Getenv() 获取。
	if err := godotenv.Load(envFile); err != nil {
		return fmt.Errorf("加载运行环境文件 %q 失败: %w", envFile, err)
	}
	return nil
}

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

// LoadConfig 在进程启动期按固定顺序加载公共配置和环境配置。
// 公共配置提供默认值，环境配置覆盖同名键；所有文件只加载一次，不进入请求热路径。
//
//	@param env
//	@return error
func LoadConfig(env string) error {
	if !hgIsSupportedEnv(Env(env)) {
		return fmt.Errorf("不支持的运行环境 %q，仅支持 debug、pre、prod", env)
	}

	// 从当前进程的环境变量中读取指定 key 的值。
	configDir := os.Getenv(hgConfigDirEnv)
	if configDir == "" {
		configDir = hgDefaultConfigDir
	}
	if err := hgLoadLocalRuntimeEnv(configDir); err != nil {
		return err
	}

	// baseConfigFiles 定义基础配置文件列表，[...]string 编译器根据初始化数量自动推断长度。
	baseConfigFiles := [...]string{"app.yaml", "log.yaml", "mysql.yaml", "redis.yaml", "kafka.yaml", "clickhouse.yaml", "tracing.yaml"}
	environmentConfigFiles := [...]string{"app.yaml", "log.yaml", "mysql.yaml", "redis.yaml", "kafka.yaml", "clickhouse.yaml"}
	configFiles := make([]string, 0, len(baseConfigFiles)+len(environmentConfigFiles))
	// 注意：这里base要在前，debug/prod/pre要在后，保证环境配置覆盖基础配置
	for _, name := range baseConfigFiles {
		// 变成类似：./config/base/app.yaml 路径的数组
		configFiles = append(configFiles, filepath.Join(configDir, "base", name))
	}
	for _, name := range environmentConfigFiles {
		// 变成类似：./config/debug/mysql.yaml 路径的数组
		configFiles = append(configFiles, filepath.Join(configDir, env, name))
	}
	// 遍历所有配置文件
	for index, configFile := range configFiles {
		// 设置配置文件路径，告诉Viper下一次读取这个配置文件
		viper.SetConfigFile(configFile)
		// 指定格式
		viper.SetConfigType("yaml")
		var err error
		if index == 0 { // Viper第一次加载 base/app.yaml创建配置树
			err = viper.ReadInConfig()
		} else { //把新的配置合并进去
			err = viper.MergeInConfig()
		}
		if err != nil {
			return fmt.Errorf("加载配置文件 %q 失败: %w", configFile, err)
		}
	}
	// 记录本次实际加载的环境，供配置读取函数安全地限定本机覆盖范围。
	viper.Set(hgLoadedEnvKey, env)

	return nil
}

func hgIsSupportedEnv(env Env) bool {
	return env == EnvDebug || env == EnvPre || env == EnvProd
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
	if port != "" {
		return port
	}
	if port = viper.GetString("server.port"); port != "" {
		return port
	}
	return "8080"
}

// GetManagementPort 返回探活和指标专用端口，部署层应仅允许负载均衡探针和 Prometheus 访问。
func GetManagementPort() string {
	port := os.Getenv("MANAGEMENT_PORT")
	if port != "" {
		return port
	}
	if port = viper.GetString("management.port"); port != "" {
		return port
	}
	return "9091"
}

// GetManagementHost 返回管理面监听地址。默认仅监听 loopback，跨容器抓取必须由部署显式放开并配合网络策略。
func GetManagementHost() string {
	host := strings.TrimSpace(os.Getenv("MANAGEMENT_HOST"))
	if host != "" {
		return host
	}
	if host = strings.TrimSpace(viper.GetString("management.host")); host != "" {
		return host
	}
	return "127.0.0.1"
}

/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-17 22:19:17
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-04 11:21:32
 * @FilePath: /MLC_GO/internal/pkg/config/hg_env_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package ConfigPackage

import (
	"os"
	"runtime"
	"strconv"
)

const (
	PROD = "prod"

	// 正式环境路径
	PROD_ENV_PATH = "./config/env_configs/hg_prod.env"
	// 开发环境路径
	DEV_ENV_PATH = "./config/env_configs/hg_debug.env"
	// 测试环境路径
	TEST_ENV_PATH = "./config/env_configs/hg_pre.env"

	// 开发电脑芯片版本【有m2Pro 和 Intel】
	DEV_COMPUTER = "M2Pro"
)

type ENVConfigModel struct {
	MySQLHost      string
	MySQLPort      string
	MySQLUser      string
	MySQLPass      string
	MySQLDB        string
	MAC_TYPE       string
	MigrateVersion int
}

/* 加载 MySQL 等基础设施配置 */
func Load() *ENVConfigModel {
	// MIGRATE_EXPECT_VERSION 是字符串，用 strconv.Atoi 转为整数
	v, err := strconv.Atoi(os.Getenv("MIGRATE_EXPECT_VERSION"))
	if err != nil {
		v = 1
	}

	return &ENVConfigModel{
		MySQLHost:      getEnvOrDefault("MYSQL_HOST", "127.0.0.1"),
		MySQLPort:      getEnvOrDefault("MYSQL_PORT", "3306"),
		MySQLUser:      getEnvOrDefault("MYSQL_USER", "root"),
		MySQLPass:      resolveMySQLPassword(),
		MySQLDB:        getEnvOrDefault("MYSQL_DB", "HG_MLC_DB"),
		MAC_TYPE:       getEnvOrDefault("MAC_TYPE", DEV_COMPUTER),
		MigrateVersion: v,
	}
}

/*
读取环境变量，若为空则返回默认值
getEnvOrDefault 不是从文件读取的，而是从操作系统的环境变量中读取;
来源是：config/env_configs/hg_debug.env
*/
func getEnvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

/*
	 获取 MySQL 密码
		判断是否 macOS + ARM64 (M1/M2/M3 芯片)
	    ↓
		是 → 优先读 MYSQL_PASSWORD_ARM，否则默认 "hh109"
				↓
		否 → 直接读 MYSQL_PASSWORD（Intel 电脑）
*/
func resolveMySQLPassword() string {
	intelPassword := os.Getenv("MYSQL_PASSWORD")

	// 仅在 macOS 下按芯片架构区分密码策略：
	// Intel(amd64) -> 使用 MYSQL_PASSWORD（可为空）
	// Apple Silicon(arm64) -> 优先 MYSQL_PASSWORD_ARM，否则回退 hh109
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		if armPassword := os.Getenv("MYSQL_PASSWORD_ARM"); armPassword != "" {
			return armPassword
		}
		return "hh109"
	}

	return intelPassword
}

/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-17 22:19:17
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-18 09:03:20
 * @FilePath: /MLC_GO/internal/config/hg_env_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package ConfigPackage

import (
	"strconv"
	"os"
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
	MySQLHost string
	MySQLPort string
	MySQLUser string
	MySQLPass string
	MySQLDB string
	MAC_TYPE string
	MigrateVersion int
}

func Load() *ENVConfigModel {
	// MIGRATE_EXPECT_VERSION 是字符串，用 strconv.Atoi 转为整数
	v, _ := strconv.Atoi(os.Getenv("MIGRATE_EXPECT_VERSION"))

	return &ENVConfigModel{
		MySQLHost: os.Getenv("MYSQL_HOST"),
		MySQLPort: os.Getenv("MYSQL_PORT"),
		MySQLUser: os.Getenv("MYSQL_USER"),
		MySQLPass: os.Getenv("MYSQL_PASSWORD"),
		MySQLDB: os.Getenv("MYSQL_DB"),
		MAC_TYPE: os.Getenv("MAC_TYPE"),
		MigrateVersion: v,
	}
}
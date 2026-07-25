/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-17 22:19:17
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-25 15:24:14
 * @FilePath: /MLC_GO/internal/pkg/config/hg_env_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package ConfigPackage

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// HGMySQLConfig 描述当前环境的 MySQL 连接和 schema 版本约束。
type HGMySQLConfig struct {
	// yaml是结构体标签，对应配置文件中的属性
	// mapstructure 是：将 map 数据映射到 struct 的库使用的标签。常见于：Viper、配置中心、环境变量转换
	// 有yaml、mapstructure表示支持2种解析方式。
	Host                 string `yaml:"host" mapstructure:"host"`
	Port                 string `yaml:"port" mapstructure:"port"`
	User                 string `yaml:"user" mapstructure:"user"`
	Password             string `yaml:"password" mapstructure:"password"`
	Database             string `yaml:"database" mapstructure:"database"`
	MigrateExpectVersion int    `yaml:"migrate_expect_version" mapstructure:"migrate_expect_version"`
}

// HGRedisConfig 描述当前环境的 Redis 网络地址。
type HGRedisConfig struct {
	Host string `yaml:"host" mapstructure:"host"`
	Port string `yaml:"port" mapstructure:"port"`
}

// GetMySQLConfig 从已加载的模块化 YAML 中读取并校验 MySQL 配置。
func GetMySQLConfig() (HGMySQLConfig, error) {
	var cfg HGMySQLConfig
	// viper.UnmarshalKey 读取 viper 配置中 mysql 节点的内容，并将其反序列化到 cfg 结构体中。这个时候结构体中的标签mapstructure起到作用了。
	if err := viper.UnmarshalKey("mysql", &cfg); err != nil {
		return cfg, fmt.Errorf("读取 MySQL 配置失败: %w", err)
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Port = strings.TrimSpace(cfg.Port)
	cfg.User = strings.TrimSpace(cfg.User)
	cfg.Database = strings.TrimSpace(cfg.Database)
	if cfg.Host == "" || cfg.User == "" || cfg.Database == "" {
		return cfg, fmt.Errorf("mysql.host、mysql.user、mysql.database 不能为空")
	}
	if err := hgValidatePort("mysql.port", cfg.Port); err != nil {
		return cfg, err
	}
	if cfg.MigrateExpectVersion < 1 {
		return cfg, fmt.Errorf("mysql.migrate_expect_version 必须大于等于 1")
	}
	return cfg, nil
}

// GetRedisConfig 从已加载的模块化 YAML 中读取并校验 Redis 配置。
func GetRedisConfig() (HGRedisConfig, error) {
	var cfg HGRedisConfig
	if err := viper.UnmarshalKey("redis", &cfg); err != nil {
		return cfg, fmt.Errorf("读取 Redis 配置失败: %w", err)
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Port = strings.TrimSpace(cfg.Port)
	if cfg.Host == "" {
		return cfg, fmt.Errorf("redis.host 不能为空")
	}
	if err := hgValidatePort("redis.port", cfg.Port); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func hgValidatePort(name string, value string) error {
	// Atoi表示十进制字符串→int
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s 必须是 1-65535 的有效端口", name)
	}
	return nil
}

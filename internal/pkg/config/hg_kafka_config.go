/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:48:12
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-17 18:01:55
 * @FilePath: /MLC_GO/internal/pkg/config/hg_kafka_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package ConfigPackage

import (
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// GetKafkaConfig 从当前已加载的 viper 配置中读取 Kafka 配置。
//
// 返回值 enabled 表示是否应该在应用启动时初始化 Kafka：
// - business.brokers 为空：Kafka 视为未启用，适合本地开发、单测和暂未接入 MQ 的环境。
// - business.brokers 非空：必须满足 Kafka 配置校验，启动期失败直接返回错误，避免请求期才暴露 MQ 不可用。
//
// 只用 business.brokers 决定 enabled，是因为当前全局 Kafka Client 以 business 集群为主：
// 1. 核心业务事件需要更高可靠性，必须先保证 business 集群可用。
// 2. log 集群是可选扩展；仅配置 log.brokers 而不配置 business.brokers 时，不应隐式启动业务 Kafka Client。
// 3. 两组 brokers 都会去除空白项，避免 YAML 中的空字符串把“未启用”误判为“配置非法”。
func GetKafkaConfig() (HGKafkaPackage.HGKafkaClusterConfig, bool, error) {
	var cfg HGKafkaPackage.HGKafkaClusterConfig
	// 读取的是config.xxx.yaml中的kafka配置
	if err := viper.UnmarshalKey("kafka", &cfg); err != nil {
		return cfg, false, fmt.Errorf("读取 Kafka 配置失败: %w", err)
	}

	cfg.Business.Brokers = trimKafkaBrokers(cfg.Business.Brokers)
	cfg.Log.Brokers = trimKafkaBrokers(cfg.Log.Brokers)

	if len(cfg.Business.Brokers) == 0 {
		return cfg, false, nil
	}

	// 这里先做静态配置校验，网络可达性由 HGInitKafka 在启动期 Ping broker 完成。
	if _, err := HGKafkaPackage.HGBuildClusterConfig(cfg.Business); err != nil {
		return cfg, true, fmt.Errorf("业务 Kafka 配置非法: %w", err)
	}

	if len(cfg.Log.Brokers) > 0 {
		if _, err := HGKafkaPackage.HGBuildClusterConfig(cfg.Log); err != nil {
			return cfg, true, fmt.Errorf("日志 Kafka 配置非法: %w", err)
		}
	}

	return cfg, true, nil
}

func trimKafkaBrokers(brokers []string) []string {
	result := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			result = append(result, broker)
		}
	}
	return result
}

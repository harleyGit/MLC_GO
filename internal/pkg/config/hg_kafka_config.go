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
// 配置来源是启动阶段 LoadConfig 根据 SERVER_ENV 合并的 config/base/kafka.yaml 和 config/<env>/kafka.yaml：
// 1. kafka.business 用于当前全局 Kafka Client，承载需要可靠确认的业务事件，是启动必填配置。
// 2. kafka.log 用于日志/埋点流量隔离；当前初始化流程尚未创建独立日志 Client，但配置存在时仍提前校验，防止上线后使用非法配置。
// 3. brokers 是 seed broker 列表，格式为 host:port；客户端连接任一 seed 后会自动发现完整集群元数据。
// 4. acks 支持 "0"、"1"、"all"；retry 小于 0 时由 Kafka 配置构建器归一化为 0。
// 5. client_id 会进入 Kafka broker 日志和指标，必须包含服务及环境标识，便于定位连接来源。
//
// 返回值 enabled 表示 business 配置是否已通过校验。business.brokers 缺失或配置非法时直接返回错误，应用不得继续启动。
//
// 只用 business.brokers 决定 enabled，是因为当前全局 Kafka Client 以 business 集群为主：
// 1. 核心业务事件需要更高可靠性，必须先保证 business 集群可用。
// 2. log 集群是可选扩展；仅配置 log.brokers 而不配置 business.brokers 时，不应隐式启动业务 Kafka Client。
// 3. 两组 brokers 都会去除空白项，避免 YAML 中的空字符串把“未启用”误判为“配置非法”。
func GetKafkaConfig() (HGKafkaPackage.HGKafkaClusterConfig, bool, error) {
	var cfg HGKafkaPackage.HGKafkaClusterConfig
	// 只反序列化 kafka 节点，避免 Kafka 配置读取与 server、db 等其他配置结构耦合。
	if err := viper.UnmarshalKey("kafka", &cfg); err != nil {
		return cfg, false, fmt.Errorf("读取 Kafka 配置失败: %w", err)
	}

	// 去除每个 broker 两侧空白并丢弃空字符串，避免 YAML 中的 " " 被当成有效地址。
	cfg.Business.Brokers = trimKafkaBrokers(cfg.Business.Brokers)
	cfg.Log.Brokers = trimKafkaBrokers(cfg.Log.Brokers)

	// 当前应用的全局 Client 使用 business 集群，因此没有有效 business seed broker 时必须快速失败。
	if len(cfg.Business.Brokers) == 0 {
		return cfg, false, fmt.Errorf("kafka.business.brokers 不能为空")
	}

	// 此处只做 brokers、acks、retry 等静态校验；网络连通性由 HGInitKafka 在启动期 Ping broker 验证。
	if _, err := HGKafkaPackage.HGBuildClusterConfig(cfg.Business); err != nil {
		return cfg, true, fmt.Errorf("业务 Kafka 配置非法: %w", err)
	}

	// log 集群允许暂不启用；一旦提供 brokers，就必须满足与 business 集群相同的基础格式约束。
	if len(cfg.Log.Brokers) > 0 {
		if _, err := HGKafkaPackage.HGBuildClusterConfig(cfg.Log); err != nil {
			return cfg, true, fmt.Errorf("日志 Kafka 配置非法: %w", err)
		}
	}

	return cfg, true, nil
}

func trimKafkaBrokers(brokers []string) []string {
	// 预分配原切片容量，清洗过程只复用容量，不在循环内反复扩容。
	result := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			result = append(result, broker)
		}
	}
	return result
}

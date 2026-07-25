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
// 1. kafka.business 用于当前全局 Kafka Client，同时承载业务事件生产与领域事件消费，是启动必填配置。
// 2. kafka.log 用于日志/埋点流量隔离；当前初始化流程尚未创建独立日志 Client，但配置存在时仍提前校验，防止上线后使用非法配置。
// 3. brokers 是 seed broker 列表，格式为 host:port；客户端连接任一 seed 后会自动发现完整集群元数据。
// 4. acks 支持 "0"、"1"、"all"；retry 小于 0 时由 Kafka 配置构建器归一化为 0。
// 5. topics/group_id 配置消费订阅与消费组；消费端关闭自动提交并由 consumeLoop 批量提交 offset。
// 6. client_id 会进入 Kafka broker 日志和指标，必须包含服务及环境标识，便于定位连接来源。
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
	cfg.Business.Topics = trimKafkaValues(cfg.Business.Topics)

	// 当前应用的全局 Client 使用 business 集群，因此没有有效 business seed broker 时必须快速失败。
	if len(cfg.Business.Brokers) == 0 {
		return cfg, false, fmt.Errorf("kafka.business.brokers 不能为空")
	}

	// 此处只做 brokers、acks、retry 等静态校验；网络连通性由 HGInitKafka 在启动期 Ping broker 验证。
	if _, err := HGKafkaPackage.HGNewBusinessProducerOpts(cfg.Business); err != nil {
		return cfg, true, fmt.Errorf("业务 Kafka 配置非法: %w", err)
	}
	if len(cfg.Business.Topics) == 0 {
		return cfg, true, fmt.Errorf("业务 Kafka 配置非法: consumer topics 不能为空")
	}
	if err := validateKafkaConsumerGroups(cfg.Business.Consumers); err != nil {
		return cfg, true, fmt.Errorf("业务 Kafka 消费组配置非法: %w", err)
	}

	// log 集群允许暂不启用；一旦提供 brokers，就必须满足与 business 集群相同的基础格式约束。
	if len(cfg.Log.Brokers) > 0 {
		if _, err := HGKafkaPackage.HGBuildClusterConfig(cfg.Log); err != nil {
			return cfg, true, fmt.Errorf("日志 Kafka 配置非法: %w", err)
		}
	}

	return cfg, true, nil
}

func validateKafkaConsumerGroups(groups HGKafkaPackage.HGConsumerGroupConfigs) error {
	configs := []struct {
		name string
		cfg  HGKafkaPackage.HGConsumerConfig
	}{
		{name: "feed", cfg: groups.Feed},
		{name: "search", cfg: groups.Search},
		{name: "statistic", cfg: groups.Statistic},
		{name: "audit", cfg: groups.Audit},
	}
	seen := make(map[string]string, len(configs))
	for _, item := range configs {
		item.cfg.GroupID = strings.TrimSpace(item.cfg.GroupID)
		if item.cfg.GroupID == "" {
			return fmt.Errorf("consumer %s group_id 不能为空", item.name)
		}
		if previous, ok := seen[item.cfg.GroupID]; ok {
			return fmt.Errorf("consumer %s 与 %s 的 group_id 重复: %s", item.name, previous, item.cfg.GroupID)
		}
		seen[item.cfg.GroupID] = item.name
	}
	return nil
}

func trimKafkaBrokers(brokers []string) []string {
	return trimKafkaValues(brokers)
}

func trimKafkaValues(values []string) []string {
	// 预分配原切片容量，清洗过程只复用容量，不在循环内反复扩容。
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

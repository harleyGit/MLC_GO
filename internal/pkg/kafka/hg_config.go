/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:34:41
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-04 17:12:35
 * @FilePath: /MLC_GO/internal/pkg/kafka/hg_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGKafkaPackage

import (
	"fmt"
	"strings"
)

const (
	// HGAcksLeader 表示 Kafka leader 写入成功即确认，适合埋点、日志等吞吐优先场景。
	HGAcksLeader = "1"
	// HGAcksAll 表示 ISR 副本全部确认后才成功，适合订单、交易等可靠性优先场景。
	HGAcksAll = "all"
	// HGAcksNone 表示生产请求写入网络后不等待 broker 响应，仅适合可丢弃数据。
	HGAcksNone = "0"
)

// HGKafkaClusterConfig 按文档拆分 business/log 两类 Kafka 集群。
//
// Business 用于高可靠业务事件，默认 acks=all；Log 用于高吞吐日志/埋点，默认 acks=1。
// 两套配置允许在生产环境中做物理集群隔离，避免日志洪峰影响核心交易链路。
type HGKafkaClusterConfig struct {
	// 业务消息集群
	Business HGClusterConfig `yaml:"business" mapstructure:"business"`
	// 埋点日志集群
	Log HGClusterConfig `yaml:"log" mapstructure:"log"`
}

// HGClusterConfig 是单个 Kafka 集群的最小生产配置。
//
// Brokers 必填，必须使用 host:port 形式；Acks 支持 "0"、"1"、"all"；Retry 小于 0 时按 0 处理。
// ClientID 建议使用服务名+环境名，便于 broker 侧观测连接来源。
type HGClusterConfig struct {
	Brokers  []string `yaml:"brokers" mapstructure:"brokers"`
	Acks     string   `yaml:"acks" mapstructure:"acks"`
	Retry    int      `yaml:"retry" mapstructure:"retry"`
	ClientID string   `yaml:"client_id" mapstructure:"client_id"`
	Topics   []string `yaml:"topics" mapstructure:"topics"`
	GroupID  string   `yaml:"group_id" mapstructure:"group_id"`
}

// HGBuildClusterConfig 校验并归一化 Kafka 集群配置。
//
// 高并发场景下，客户端启动时快速失败比请求期反复报错更可控；因此这里在初始化阶段拦截空 broker、空白 broker、非法 acks。
func HGBuildClusterConfig(cfg HGClusterConfig) (HGClusterConfig, error) {
	brokers := make([]string, 0, len(cfg.Brokers))
	for _, broker := range cfg.Brokers {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			brokers = append(brokers, broker)
		}
	}

	if len(brokers) == 0 {
		return HGClusterConfig{}, fmt.Errorf("kafka brokers cannot be empty")
	}

	acks := strings.TrimSpace(strings.ToLower(cfg.Acks))
	if acks == "" {
		acks = HGAcksAll
	}

	switch acks {
	case HGAcksNone, HGAcksLeader, HGAcksAll:
	default:
		return HGClusterConfig{}, fmt.Errorf("unsupported kafka acks %q", cfg.Acks)
	}

	retry := cfg.Retry
	if retry < 0 {
		retry = 0
	}

	return HGClusterConfig{
		Brokers:  brokers,
		Acks:     acks,
		Retry:    retry,
		ClientID: strings.TrimSpace(cfg.ClientID),
		Topics:   hgTrimNonEmptyStrings(cfg.Topics),
		GroupID:  strings.TrimSpace(cfg.GroupID),
	}, nil
}

func hgTrimNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

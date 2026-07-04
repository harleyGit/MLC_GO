package main

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"fmt"

	"MLC_GO/internal/pkg/logHG"
)

// kafkaCloser 表示应用关闭阶段需要释放的 Kafka 资源。
type kafkaCloser func()

// initKafkaIfConfigured 按当前运行配置决定是否初始化 Kafka。
//
// Kafka 在本工程中按“可选基础设施”接入：未配置 business.brokers 时不阻断启动；一旦配置了 broker，初始化失败就返回错误。
// 这样既能保证本地开发/单测无 Kafka 也可运行，又能让生产配置错误在启动期快速失败。
//
// 返回 kafkaCloser 而不是在 main 中直接引用 HGCloseKafka，是为了让主应用只关心“是否有资源需要关闭”：
// 1. nil 表示 Kafka 未启用，Close 阶段无需做任何事。
// 2. 非 nil 表示 Kafka 已完成初始化，Close 阶段必须 flush 并关闭 client。
// 3. 初始化失败不会返回 closer，调用方应按启动失败路径回滚前置依赖。
func initKafkaIfConfigured() (kafkaCloser, error) {
	cfg, enabled, err := ConfigPackage.GetKafkaConfig()
	if err != nil {
		return nil, err
	}
	if !enabled {
		// brokers 为空时视为显式未启用，而不是配置错误；这样 debug/pre/prod 模板可以先保留 Kafka 段。
		logHG.DebugInfo("Kafka未配置，跳过初始化")
		return nil, nil
	}

	// HGInitKafka 内部会做配置归一化、client 创建和 broker Ping；这里不再重复做网络探测。
	if err := HGKafkaPackage.HGInitKafka(cfg); err != nil {
		return nil, fmt.Errorf("Kafka初始化失败: %w", err)
	}

	logHG.DebugInfo("Kafka初始化完成")
	return HGKafkaPackage.HGCloseKafka, nil
}

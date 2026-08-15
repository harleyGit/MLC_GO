// Package runtime 组装 crawler 平台客户端和任务管理器，供独立命令与主应用复用。
package runtime

import (
	"fmt"
	"time"

	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	CrawlerSpiderPackage "MLC_GO/internal/modules/crawler/spider"
)

// HGBilibiliRuntimeConfig 是构造 Bilibili crawler 运行时所需的最小配置。
// 是否启动由上层入口决定，运行时工厂只负责构造已启用的 manager。
type HGBilibiliRuntimeConfig struct {
	Interval      time.Duration
	Timeout       time.Duration
	MaxItems      int
	RetryCount    int
	RatePerSecond float64
	UserAgent     string
}

// NewHGBilibiliManager 创建 Bilibili 平台客户端和串行任务管理器。
// 独立 cmd 与主应用必须通过该工厂装配，避免两种启动方式出现不同的限流、超时或重试行为。
func NewHGBilibiliManager(config HGBilibiliRuntimeConfig) (*CrawlerSpiderPackage.HGManager, error) {
	platform, err := CrawlerPlatformPackage.NewHGBilibiliPlatform(nil, CrawlerPlatformPackage.HGBilibiliConfig{
		UserAgent:      config.UserAgent,
		RequestTimeout: config.Timeout,
		MaxItems:       config.MaxItems,
		RetryCount:     config.RetryCount,
		RatePerSecond:  config.RatePerSecond,
	})
	if err != nil {
		return nil, fmt.Errorf("creating bilibili crawler platform: %w", err)
	}
	manager, err := CrawlerSpiderPackage.NewHGManager(platform, config.Interval, config.Timeout)
	if err != nil {
		return nil, fmt.Errorf("creating bilibili crawler manager: %w", err)
	}
	return manager, nil
}

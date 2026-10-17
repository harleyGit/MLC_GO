package ConfigPackage

import (
	CrawlerEventPackage "MLC_GO/internal/events/crawler"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"os"
)

// HGCrawlerVideoPipelineEnabled 显式控制发现事件写入；先部署迁移和消费者，再打开生产端。
// 进程环境开关不读取凭据，未设置时保留既有推荐采集行为。
func HGCrawlerVideoPipelineEnabled() bool {
	return os.Getenv("CRAWLER_VIDEO_PIPELINE_ENABLED") == "true"
}

// hgConfigureCrawlerConsumers 开关启用时提供环境隔离的默认组，显式配置仍可指定组名。
func hgConfigureCrawlerConsumers(cfg *HGKafkaPackage.HGClusterConfig) {
	if !HGCrawlerVideoPipelineEnabled() {
		return
	}
	env := os.Getenv("SERVER_ENV")
	if env == "" {
		env = "debug"
	}
	for _, item := range []struct {
		config *HGKafkaPackage.HGConsumerConfig
		stage  string
		name   string
	}{
		{&cfg.Consumers.CrawlerFetch, CrawlerEventPackage.HGDiscovered, "fetch"},
		{&cfg.Consumers.CrawlerClean, CrawlerEventPackage.HGRaw, "clean"},
		{&cfg.Consumers.CrawlerStore, CrawlerEventPackage.HGCleaned, "store"},
		{&cfg.Consumers.CrawlerHistory, CrawlerEventPackage.HGStored, "history"},
	} {
		item.config.Enabled = true
		if item.config.GroupID == "" {
			item.config.GroupID = "mlc-go-" + env + "-crawler-" + item.name
		}
		if item.config.ClientID == "" {
			item.config.ClientID = item.config.GroupID
		}
		item.config.Topics = []string{CrawlerEventPackage.HGTopicPrefix + item.stage}
	}
}

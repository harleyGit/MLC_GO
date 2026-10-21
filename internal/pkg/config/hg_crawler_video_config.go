/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-10-17 14:38:10
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-10-21 11:18:17
 * @FilePath: /MLC_GO/internal/pkg/config/hg_crawler_video_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
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

// hgConfigureCrawlerConsumers 为视频爬虫流水线自动配置 4 个 Kafka 消费者，分别负责视频抓取、数据清洗、数据存储和历史记录处理。
// 它只负责配置消费者，并不会直接启动 Kafka 消费循环。
// 总结： 这个方法本质上是一个 Kafka 消费者配置初始化器，通过统一配置 4 个消费者，把视频爬虫拆分成 发现 → 抓取 → 清洗 → 存储 → 历史处理 的事件驱动流水线，为后续的独立部署、扩容和故障隔离提供基础。
/*执行流程：
B站视频被发现
      |
      v
Kafka: discovered
      |
      v
[CrawlerFetch] 抓取视频详情
      |
      v
Kafka: raw
      |
      v
[CrawlerClean] 清洗原始数据
      |
      v
Kafka: cleaned
      |
      v
[CrawlerStore] 保存清洗后的数据
      |
      v
Kafka: stored
      |
      v
[CrawlerHistory] 处理历史数据
这是一种典型的 Kafka 事件驱动流水线（Event-Driven Pipeline） 架构。
*/
func hgConfigureCrawlerConsumers(cfg *HGKafkaPackage.HGClusterConfig) {
	// 是否开启爬虫功能
	if !HGCrawlerVideoPipelineEnabled() {
		return
	}
	env := os.Getenv("SERVER_ENV")
	if env == "" {
		env = "debug"
	}
	// 定义 4 个消费者，这里使用的是 匿名结构体切片 + for range 遍历
	/* 为什么要设计 4 个消费者，而不是 1 个？
	因为四个阶段的处理能力和资源需求不同。

	例如：
	- Fetch：需要访问 B 站接口，受网络和限流影响。
	- Clean：主要负责 JSON 解析、字段校验、数据清洗。
	- Store：主要负责数据库写入。
	- History：负责保存或处理历史数据，具体由其 Handler 决定。
	分成四个消费者后，可以独立扩容。

	假设 Fetch 阶段出现积压，可以增加 Fetch 实例，而不需要增加其他阶段的实例。
	*/
	for _, item := range []struct {
		config *HGKafkaPackage.HGConsumerConfig // 对应消费者的配置对象指针
		stage  string                           // 需要订阅的事件阶段
		name   string                           // 消费者名称，用于生成GroupID
	}{
		// 表示配置 CrawlerFetch 消费者，让它订阅视频发现事件。
		{&cfg.Consumers.CrawlerFetch, CrawlerEventPackage.HGDiscovered, "fetch"},
		{&cfg.Consumers.CrawlerClean, CrawlerEventPackage.HGRaw, "clean"},
		{&cfg.Consumers.CrawlerStore, CrawlerEventPackage.HGCleaned, "store"},
		{&cfg.Consumers.CrawlerHistory, CrawlerEventPackage.HGStored, "history"},
	} {
		// 将当前消费者标记为启用状态。
		// 注意：Enabled = true 不代表消费者已经开始消费 Kafka 消息，还需要后续启动逻辑。
		item.config.Enabled = true
		// 配置 GroupID（消费者组 ID）
		/* GroupID 有什么作用？

		Kafka 使用 GroupID 标识消费者组。

		例如部署 3 个 Fetch 实例：
		Kafka Topic: crawler.discovered
		       |
		       v
		GroupID: mlc-go-debug-crawler-fetch
		       |
		       +---- Fetch实例 A
		       |
		       +---- Fetch实例 B
		       |
		       +---- Fetch实例 C


		三个实例属于同一个消费组，会共同分担 Topic 分区的消息。

		同一个消费组内，一个分区在同一时刻通常只分配给一个消费者实例。

		因此，不是三个实例都重复处理每一条消息，而是由 Kafka 分配处理任务。
		*/
		if item.config.GroupID == "" {
			//如果没有配置 GroupID，就自动生成
			item.config.GroupID = "mlc-go-" + env + "-crawler-" + item.name
		}
		// 配置 ClientID
		if item.config.ClientID == "" {
			// GroupID	确定消费组，影响消息分配和消费位点管理
			// ClientID	标识 Kafka 客户端，方便日志和监控
			// TODO：注意：如果部署多个实例，当前代码可能使它们使用相同的 ClientID。生产环境建议添加实例标识，便于排查问题。
			item.config.ClientID = item.config.GroupID
		}
		// 配置订阅的Topic
		// 这里的 Topic 名称是示例，实际取决于 HGTopicPrefix 和各阶段常量的定义。
		// 每个消费者只负责处理自己对应阶段的消息，实现职责分离。
		item.config.Topics = []string{CrawlerEventPackage.HGTopicPrefix + item.stage}
	}
}

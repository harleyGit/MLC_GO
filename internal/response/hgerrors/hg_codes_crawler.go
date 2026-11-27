package hgerrors

const (
	HGCrawlerTaskNotFound       Code = 1701001
	HGCrawlerTaskAlreadyRunning Code = 1701002
	HGCrawlerTaskNotRunning     Code = 1701003
	HGCrawlerTaskCancelled      Code = 1701004
	HGCrawlerSourceUnavailable  Code = 1702001
	HGCrawlerSourceRateLimited  Code = 1702002
	HGCrawlerSourceAccessDenied Code = 1702003
	HGCrawlerInvalidBvid        Code = 1703001
	HGCrawlerStageConflict      Code = 1703002
	HGCrawlerCleanFailed        Code = 1703003
	HGCrawlerStoreFailed        Code = 1703004
	HGCrawlerKafkaPublishFailed Code = 1704001
	HGCrawlerEventDuplicated    Code = 1704002
)

var hgCrawler = []Definition{
	{HGCrawlerTaskNotFound, "CRAWLER_TASK_NOT_FOUND", "任务不存在", 404, false},
	{HGCrawlerTaskAlreadyRunning, "CRAWLER_TASK_ALREADY_RUNNING", "任务已运行", 409, false},
	{HGCrawlerTaskNotRunning, "CRAWLER_TASK_NOT_RUNNING", "任务未运行", 409, false},
	{HGCrawlerTaskCancelled, "CRAWLER_TASK_CANCELLED", "任务已取消", 409, false},
	{HGCrawlerSourceUnavailable, "CRAWLER_SOURCE_UNAVAILABLE", "数据源暂不可用", 503, true},
	{HGCrawlerSourceRateLimited, "CRAWLER_SOURCE_RATE_LIMITED", "上游限流", 503, true},
	{HGCrawlerSourceAccessDenied, "CRAWLER_SOURCE_ACCESS_DENIED", "数据源拒绝访问", 403, false},
	{HGCrawlerInvalidBvid, "CRAWLER_INVALID_BVID", "视频 BV 号无效", 400, false},
	{HGCrawlerStageConflict, "CRAWLER_STAGE_CONFLICT", "流水线阶段不匹配", 409, false},
	{HGCrawlerCleanFailed, "CRAWLER_CLEAN_FAILED", "数据清洗失败", 500, false},
	{HGCrawlerStoreFailed, "CRAWLER_STORE_FAILED", "数据存储失败", 500, false},
	{HGCrawlerKafkaPublishFailed, "CRAWLER_KAFKA_PUBLISH_FAILED", "事件投递失败", 503, true},
	{HGCrawlerEventDuplicated, "CRAWLER_EVENT_DUPLICATED", "事件重复且内容冲突", 409, false},
}

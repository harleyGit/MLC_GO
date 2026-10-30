package SQLQueriesPackage

const (
	// HGGetSpiderSwitchSQL 按固定主键点查期望状态，worker 每隔两秒读取一次；单实例运行，避免多副本重复采集。
	HGGetSpiderSwitchSQL = `SELECT desired_running FROM crawler_spider_switches WHERE spider_id='bilibili'`
	// HGLockSpiderSwitchSQL 仅在短事务内锁定单行，序列化并发启停和审计写入。
	HGLockSpiderSwitchSQL = HGGetSpiderSwitchSQL + ` FOR UPDATE`
	// HGUpdateSpiderSwitchSQL 事务内更新权威期望状态；使用数据库 UTC 会话时间记录切换时刻。
	HGUpdateSpiderSwitchSQL = `UPDATE crawler_spider_switches SET desired_running=?,updated_by=?,updated_at=UTC_TIMESTAMP(6) WHERE spider_id='bilibili'`
	// HGInsertSpiderSwitchEventSQL 每次实际改变期望值时写一条审计；不记录重复请求或未提交的尝试。
	HGInsertSpiderSwitchEventSQL = `INSERT INTO crawler_spider_switch_events (spider_id,desired_running,actor_id,created_at) VALUES ('bilibili',?,?,UTC_TIMESTAMP(6))`
)

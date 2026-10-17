package SQLQueriesPackage

const (
	// HGGetCrawlerMigrationVersionSQL 供debug验收前检查迁移版本，仅读取单行元数据。
	HGGetCrawlerMigrationVersionSQL = `SELECT version,dirty FROM schema_migrations LIMIT 1`
	// HGInsertVideoCollectionSQL 在推荐事务中创建一条采集身份，与 discovered Outbox 同提交。
	HGInsertVideoCollectionSQL = `INSERT INTO crawler_video_collections (collection_id,bvid,task_id,run_id,discovered_at) VALUES (?,?,?,?,?)`
	// HGGetVideoCollectionSQL 按主键读取阶段；事务版本加行锁，固定先锁 collection 再锁详情。
	HGGetVideoCollectionSQL     = `SELECT bvid,status FROM crawler_video_collections WHERE collection_id=?`
	HGLockVideoCollectionSQL    = HGGetVideoCollectionSQL + ` FOR UPDATE`
	HGAdvanceVideoCollectionSQL = `UPDATE crawler_video_collections SET status=?,error_message=? WHERE collection_id=? AND status=?`
	// HGUpsertVideoDetailSQL 按主键锁定单视频，时间与UUID组成稳定版本；赋值顺序保证比较使用旧版本。
	HGUpsertVideoDetailSQL = `INSERT INTO crawler_video_details (bvid,snapshot,collected_at,collection_id) VALUES (?,?,?,?)
ON DUPLICATE KEY UPDATE
snapshot=IF((VALUES(collected_at),VALUES(collection_id))>(collected_at,collection_id),VALUES(snapshot),snapshot),
collection_id=IF(VALUES(collected_at)>=collected_at,GREATEST(IF(VALUES(collected_at)>collected_at,'',collection_id),VALUES(collection_id)),collection_id),
collected_at=GREATEST(collected_at,VALUES(collected_at))`
	HGUpsertVideoStatsSQL = `INSERT INTO crawler_video_stats_latest (bvid,snapshot,collected_at,collection_id) VALUES (?,?,?,?)
ON DUPLICATE KEY UPDATE
snapshot=IF((VALUES(collected_at),VALUES(collection_id))>(collected_at,collection_id),VALUES(snapshot),snapshot),
collection_id=IF(VALUES(collected_at)>=collected_at,GREATEST(IF(VALUES(collected_at)>collected_at,'',collection_id),VALUES(collection_id)),collection_id),
collected_at=GREATEST(collected_at,VALUES(collected_at))`
	// HGGetVideoDetailSQL 两张表按同类型主键一对一点查；一个语句保证详情与统计来自同一提交快照。
	HGGetVideoDetailSQL = `SELECT d.snapshot,s.snapshot FROM crawler_video_details d JOIN crawler_video_stats_latest s ON s.bvid=d.bvid WHERE d.bvid=?`
	// HGGetVideoCollectionStatusSQL 使用 idx_video_recent，有界读取最新一次状态，无 COUNT/深分页。
	HGGetVideoCollectionStatusSQL = `SELECT status,error_message FROM crawler_video_collections WHERE bvid=? ORDER BY discovered_at DESC,collection_id DESC LIMIT 1`
)

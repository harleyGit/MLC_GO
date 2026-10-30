package SQLQueriesPackage

// HGBackfillVideoPageSQL 按主键游标扫描最多100条历史列表；详情表按主键点查，跳过已入库记录。
// 仅供显式授权的补采命令使用，不修改推荐摘要及任务关联。
const HGBackfillVideoPageSQL = `SELECT e.id, e.platform, e.content_id,
 (EXISTS(SELECT 1 FROM crawler_video_details d WHERE d.bvid=e.content_id) OR
 COALESCE((SELECT c.status IN ('pending','raw','cleaned') FROM crawler_video_collections c WHERE c.bvid=e.content_id ORDER BY c.discovered_at DESC,c.collection_id DESC LIMIT 1),0))
 FROM crawler_external_contents e WHERE e.id > ? ORDER BY e.id ASC LIMIT 100`

// HGBackfillTaskVideoPageSQL 与任务内容接口使用同一关联表；按(task_definition_id,id)游标扫100条，
// 通过内容主键与详情主键点查，已入库或在途的视频不再次触发抓取。
const HGBackfillTaskVideoPageSQL = `SELECT a.id, e.platform, e.content_id,
 (EXISTS(SELECT 1 FROM crawler_video_details d WHERE d.bvid=e.content_id) OR
 COALESCE((SELECT c.status IN ('pending','raw','cleaned') FROM crawler_video_collections c WHERE c.bvid=e.content_id ORDER BY c.discovered_at DESC,c.collection_id DESC LIMIT 1),0))
 FROM crawler_task_external_contents a JOIN crawler_external_contents e ON e.id=a.external_content_id
 WHERE a.task_definition_id=? AND a.id>? ORDER BY a.id ASC LIMIT 100`

// HGBackfillVideoVerifySQL 按列表主键每页最多100条检查入库；详情/统计主键连接，状态使用idx_video_recent限1。
const HGBackfillVideoVerifySQL = `SELECT e.id, e.content_id, d.bvid IS NOT NULL AND s.bvid IS NOT NULL,
 COALESCE((SELECT CONCAT(c.status,IF(c.error_message='', '', CONCAT(':',c.error_message))) FROM crawler_video_collections c WHERE c.bvid=e.content_id ORDER BY c.discovered_at DESC,c.collection_id DESC LIMIT 1),'missing')
 FROM crawler_external_contents e
 LEFT JOIN crawler_video_details d ON d.bvid=e.content_id AND e.platform='bilibili'
 LEFT JOIN crawler_video_stats_latest s ON s.bvid=d.bvid
 WHERE e.id > ? ORDER BY e.id ASC LIMIT 100`

// HGBackfillTaskVideoVerifySQL 按任务关联索引逐页核对详情与统计，只读；游标是关联ID，不是内容ID。
const HGBackfillTaskVideoVerifySQL = `SELECT a.id, e.content_id, d.bvid IS NOT NULL AND s.bvid IS NOT NULL,
 COALESCE((SELECT CONCAT(c.status,IF(c.error_message='', '', CONCAT(':',c.error_message))) FROM crawler_video_collections c WHERE c.bvid=e.content_id ORDER BY c.discovered_at DESC,c.collection_id DESC LIMIT 1),'missing')
 FROM crawler_task_external_contents a JOIN crawler_external_contents e ON e.id=a.external_content_id
 LEFT JOIN crawler_video_details d ON d.bvid=e.content_id AND e.platform='bilibili'
 LEFT JOIN crawler_video_stats_latest s ON s.bvid=d.bvid
 WHERE a.task_definition_id=? AND a.id>? ORDER BY a.id ASC LIMIT 100`

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

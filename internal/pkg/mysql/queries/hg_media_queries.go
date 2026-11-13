package SQLQueriesPackage

const (
	// 启用媒体模块前校验各表和必需增量字段；仅启动期执行，不扫描业务数据。
	HGMediaAssetSchemaSQL   = `SELECT asset_id,sealed_key FROM media_assets LIMIT 0`
	HGMediaSessionSchemaSQL = `SELECT upload_id,next_check_at,lease_token FROM media_upload_sessions LIMIT 0`
	HGMediaQuotaSchemaSQL   = `SELECT owner_id FROM media_upload_quota LIMIT 0`
	// HGMediaQuotaCreateSQL 为单用户申请串行化建立短事务锁，不扫描整张媒体表；按 owner 主键定位。
	HGMediaQuotaCreateSQL = `INSERT IGNORE INTO media_upload_quota (owner_id) VALUES (?)`
	HGMediaQuotaLockSQL   = `SELECT owner_id FROM media_upload_quota WHERE owner_id = ? FOR UPDATE`
	// HGMediaReservationSQL 由 owner/status/expires 索引过滤，最多 20 个活动会话；禁止按全表统计配额。
	HGMediaReservationSQL      = `SELECT expected_size FROM media_upload_sessions WHERE owner_id = ? AND status IN ('pending','processing') AND expires_at > NOW(3) LIMIT 21`
	HGMediaFindIdempotencySQL  = `SELECT upload_id, asset_id, purpose, mime_type, expected_size, expected_sha256, object_key, status, expires_at FROM media_upload_sessions WHERE owner_id = ? AND idempotency_key = ?`
	HGMediaInsertAssetSQL      = `INSERT INTO media_assets (asset_id, owner_id, purpose, bucket, object_key, mime_type, file_size, sha256, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', NOW(3), NOW(3))`
	HGMediaInsertSessionSQL    = `INSERT INTO media_upload_sessions (upload_id, asset_id, owner_id, idempotency_key, purpose, mime_type, expected_size, expected_sha256, object_key, status, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', DATE_ADD(NOW(3), INTERVAL 24 HOUR), NOW(3))`
	HGMediaLockSessionSQL      = `SELECT asset_id, owner_id, purpose, mime_type, expected_size, object_key, status, expires_at FROM media_upload_sessions WHERE upload_id = ? FOR UPDATE`
	HGMediaFindOwnedSessionSQL = `SELECT asset_id, object_key, expected_size, status, expires_at FROM media_upload_sessions WHERE upload_id = ? AND owner_id = ?`
	HGMediaProcessSessionSQL   = `UPDATE media_upload_sessions SET status = 'processing', completed_at = NOW(3), next_check_at = NOW(3) WHERE upload_id = ? AND status = 'pending'`
	HGMediaProcessAssetSQL     = `UPDATE media_assets SET status = 'processing', updated_at = NOW(3) WHERE asset_id = ? AND status = 'pending'`
	HGMediaOwnerAssetSQL       = `SELECT purpose, mime_type, file_size, status FROM media_assets WHERE asset_id = ? AND owner_id = ?`
	HGMediaOwnerDownloadSQL    = `SELECT purpose,status,sealed_key FROM media_assets WHERE asset_id=? AND owner_id=?`
	HGMediaDeleteSchemaSQL     = `SELECT job_id,finalized_at FROM media_delete_jobs LIMIT 0`
	HGMediaRefsSchemaSQL       = `SELECT ref_id FROM media_asset_refs LIMIT 0`
	// 审核由 (status,next_check_at,upload_id) 索引驱动；单行 SKIP LOCKED，外部 I/O 发生在租约事务提交后。
	HGMediaReviewClaimSQL  = `SELECT upload_id,asset_id,purpose,object_key,mime_type,expected_size,expected_sha256,attempts FROM media_upload_sessions WHERE status='processing' AND next_check_at<=NOW(3) AND expires_at>NOW(3) ORDER BY next_check_at,upload_id LIMIT 1 FOR UPDATE SKIP LOCKED`
	HGMediaReviewLeaseSQL  = `UPDATE media_upload_sessions SET lease_token=?, attempts=attempts+1, next_check_at=DATE_ADD(NOW(3), INTERVAL 330 SECOND) WHERE upload_id=? AND status='processing'`
	HGMediaReviewLockSQL   = `SELECT status,lease_token FROM media_upload_sessions WHERE upload_id=? FOR UPDATE`
	HGMediaReviewFinishSQL = `UPDATE media_upload_sessions SET status=?,lease_token=NULL,next_check_at=DATE_ADD(NOW(3), INTERVAL ? SECOND) WHERE upload_id=? AND status='processing' AND lease_token=?`
	HGMediaReviewAssetSQL  = `UPDATE media_assets SET status=?,sealed_key=?,width=?,height=?,duration_ms=?,updated_at=NOW(3) WHERE asset_id=? AND status='processing'`
	// 过期检查仅使用 (status,expires_at,upload_id) 索引，单次只锁一个会话；不得在事务内删除 S3 对象。
	HGMediaExpireClaimSQL      = `SELECT upload_id,asset_id,object_key,status FROM media_upload_sessions WHERE status='pending' AND expires_at<=NOW(3) AND (lease_token IS NULL OR next_check_at<=NOW(3)) ORDER BY expires_at,upload_id LIMIT 1 FOR UPDATE SKIP LOCKED`
	HGMediaExpireProcessingSQL = `SELECT upload_id,asset_id,object_key,status FROM media_upload_sessions WHERE status='processing' AND expires_at<=NOW(3) AND (lease_token IS NULL OR next_check_at<=NOW(3)) ORDER BY expires_at,upload_id LIMIT 1 FOR UPDATE SKIP LOCKED`
	HGMediaExpireFailedSQL     = `SELECT upload_id,asset_id,object_key,status FROM media_upload_sessions WHERE status='failed' AND expires_at<=NOW(3) AND (lease_token IS NULL OR next_check_at<=NOW(3)) ORDER BY expires_at,upload_id LIMIT 1 FOR UPDATE SKIP LOCKED`
	HGMediaExpireReadySQL      = `SELECT upload_id,asset_id,object_key,status FROM media_upload_sessions WHERE status='ready' AND expires_at<=NOW(3) AND (lease_token IS NULL OR next_check_at<=NOW(3)) ORDER BY expires_at,upload_id LIMIT 1 FOR UPDATE SKIP LOCKED`
	HGMediaExpireAssetLockSQL  = `SELECT status,sealed_key FROM media_assets WHERE asset_id=? FOR UPDATE`
	HGMediaHasRefSQL           = `SELECT 1 FROM media_asset_refs WHERE asset_id=? LIMIT 1`
	HGMediaExpireAttachSQL     = `UPDATE media_upload_sessions SET status='attached',lease_token=NULL WHERE upload_id=? AND status='ready'`
	HGMediaExpireAssetSQL      = `UPDATE media_assets SET status='deleting',updated_at=NOW(3) WHERE asset_id=? AND status IN ('pending','processing','failed','ready')`
	HGMediaExpireSessionSQL    = `UPDATE media_upload_sessions SET status='deleting',lease_token=NULL WHERE upload_id=? AND status IN ('pending','processing','failed','ready')`
	HGMediaDeleteQueueSQL      = `INSERT INTO media_delete_jobs (asset_id,bucket,object_key,execute_after,updated_at) VALUES (?,?,?,NOW(3),NOW(3))`
	// 删除任务由 (status,execute_after,job_id) 索引驱动；租约提交后才允许对象存储 I/O。
	HGMediaDeleteClaimSQL        = `SELECT job_id,asset_id,bucket,object_key,attempts FROM media_delete_jobs WHERE status='pending' AND execute_after<=NOW(3) ORDER BY execute_after,job_id LIMIT 1 FOR UPDATE SKIP LOCKED`
	HGMediaDeleteReclaimSQL      = `SELECT job_id,asset_id,bucket,object_key,attempts FROM media_delete_jobs WHERE status='leased' AND execute_after<=NOW(3) ORDER BY execute_after,job_id LIMIT 1 FOR UPDATE SKIP LOCKED`
	HGMediaDeleteLeaseSQL        = `UPDATE media_delete_jobs SET status='leased',lease_token=?,attempts=attempts+1,execute_after=DATE_ADD(NOW(3),INTERVAL 120 SECOND),updated_at=NOW(3) WHERE job_id=? AND status IN ('pending','leased')`
	HGMediaDeleteFinishSQL       = `UPDATE media_delete_jobs SET status=?,lease_token=NULL,execute_after=DATE_ADD(NOW(3),INTERVAL ? SECOND),updated_at=NOW(3) WHERE job_id=? AND status='leased' AND lease_token=?`
	HGMediaDeleteRemainingSQL    = `SELECT 1 FROM media_delete_jobs WHERE asset_id=? AND status<>'done' LIMIT 1`
	HGMediaDeleteFinalAssetSQL   = `UPDATE media_assets SET status='deleted',updated_at=NOW(3) WHERE asset_id=? AND status='deleting'`
	HGMediaDeleteFinalSessionSQL = `UPDATE media_upload_sessions SET status='deleted' WHERE asset_id=? AND status='deleting'`
	// (status,finalized_at,job_id) 游标只访问未核对的已删任务，不在亿级资源表上轮询。
	HGMediaDeleteRecoverSQL = `SELECT asset_id FROM media_delete_jobs WHERE status='done' AND finalized_at IS NULL ORDER BY job_id LIMIT 1`
	HGMediaMarkFinalizedSQL = `UPDATE media_delete_jobs SET finalized_at=NOW(3) WHERE asset_id=? AND status='done' AND finalized_at IS NULL`
	// 业务服务须先锁定/核实业务主表；以下只借助 (biz_type,biz_id,slot) 唯一键锁定单个业务槽。
	HGMediaReferenceSlotSQL    = `SELECT asset_id FROM media_asset_refs WHERE biz_type=? AND biz_id=? AND slot=? FOR UPDATE`
	HGMediaReferenceSessionSQL = `SELECT status FROM media_upload_sessions WHERE asset_id=? FOR UPDATE`
	HGMediaReferenceAssetSQL   = `SELECT owner_id,purpose,status FROM media_assets WHERE asset_id=? FOR UPDATE`
	HGMediaReferenceInsertSQL  = `INSERT INTO media_asset_refs (asset_id,owner_id,biz_type,biz_id,slot,created_at) VALUES (?,?,?,?,?,NOW(3))`
	HGMediaReferenceUpdateSQL  = `UPDATE media_asset_refs SET asset_id=?,owner_id=? WHERE biz_type=? AND biz_id=? AND slot=? AND asset_id=?`
	HGMediaReferenceDeleteSQL  = `DELETE FROM media_asset_refs WHERE biz_type=? AND biz_id=? AND slot=? AND asset_id=?`
	HGMediaReferenceAttachSQL  = `UPDATE media_upload_sessions SET status='attached' WHERE asset_id=? AND status IN ('ready','attached')`
	// 解绑后保留 24 小时保护期；只有在最后一个引用消失后才使会话重新参与有界过期回收。
	HGMediaReferenceReleaseSQL = `UPDATE media_upload_sessions SET status='ready',expires_at=DATE_ADD(NOW(3),INTERVAL 24 HOUR) WHERE asset_id=? AND status='attached'`
	// 仅供显式 opt-in 的独占 debug 测试使用：未清空新媒体表时拒绝运行全局 worker 验收。
	HGMediaDebugEmptySQL          = `SELECT EXISTS(SELECT 1 FROM media_assets LIMIT 1)+EXISTS(SELECT 1 FROM media_delete_jobs LIMIT 1)`
	HGMediaDebugExpireSQL         = `UPDATE media_upload_sessions SET expires_at=DATE_SUB(NOW(3),INTERVAL 1 SECOND) WHERE upload_id=? AND asset_id=? AND owner_id=? AND status='ready'`
	HGMediaDebugAssetStatusSQL    = `SELECT status FROM media_assets WHERE asset_id=? AND owner_id=?`
	HGMediaDebugInspectSQL        = `SELECT asset_id,owner_id,status,object_key FROM media_assets ORDER BY asset_id LIMIT 2`
	HGMediaDebugInspectSessionSQL = `SELECT upload_id,status FROM media_upload_sessions WHERE asset_id=? LIMIT 2`
	// 仅清理经人工检查为单条新测试记录的夹具；必须先由正常 worker 标记已删除且两个 S3 Key 不存在。
	HGMediaDebugExpirePendingSQL = `UPDATE media_upload_sessions SET expires_at=DATE_SUB(NOW(3),INTERVAL 1 SECOND) WHERE upload_id=? AND asset_id=? AND owner_id=? AND status='pending'`
	HGMediaDebugDoneJobsSQL      = `SELECT COUNT(*) FROM media_delete_jobs WHERE asset_id=? AND status='done'`
	HGMediaDebugDeleteJobsSQL    = `DELETE FROM media_delete_jobs WHERE asset_id=? AND status='done'`
	HGMediaDebugDeleteSessionSQL = `DELETE FROM media_upload_sessions WHERE upload_id=? AND asset_id=? AND owner_id=? AND status='deleted'`
	HGMediaDebugDeleteAssetSQL   = `DELETE FROM media_assets WHERE asset_id=? AND owner_id=? AND status='deleted'`
	HGMediaDebugDeleteQuotaSQL   = `DELETE FROM media_upload_quota WHERE owner_id=?`
	// 旧来源只读样本：均走主键顺序，100 行上限，不按未索引的 URL/路径过滤，不读取文件字节或输出隐私。
	HGMediaLegacyAvatarSampleSQL = `SELECT user_id,avatar_url FROM users ORDER BY id LIMIT 100`
	HGMediaLegacyMallSampleSQL   = `SELECT user_id,product_id,storage_key,status FROM mall_product_image ORDER BY id LIMIT 100`
	HGMediaLegacyVideoSampleSQL  = `SELECT user_id,submission_id,file_path,upload_status FROM video_files ORDER BY id LIMIT 100`
	HGMediaLegacyIMSampleSQL     = `SELECT owner_id,conversation_id,object_key,immutable_key,state FROM im_private_attachment ORDER BY id LIMIT 100`
)

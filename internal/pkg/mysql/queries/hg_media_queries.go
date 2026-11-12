package SQLQueriesPackage

const (
	// 启用媒体模块前校验迁移中的三个表都存在；仅启动期执行，不扫描业务数据。
	HGMediaAssetSchemaSQL   = `SELECT asset_id FROM media_assets LIMIT 0`
	HGMediaSessionSchemaSQL = `SELECT upload_id FROM media_upload_sessions LIMIT 0`
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
	HGMediaProcessSessionSQL   = `UPDATE media_upload_sessions SET status = 'processing', completed_at = NOW(3) WHERE upload_id = ? AND status = 'pending'`
	HGMediaProcessAssetSQL     = `UPDATE media_assets SET status = 'processing', updated_at = NOW(3) WHERE asset_id = ? AND status = 'pending'`
	HGMediaOwnerAssetSQL       = `SELECT purpose, mime_type, file_size, status FROM media_assets WHERE asset_id = ? AND owner_id = ?`
)

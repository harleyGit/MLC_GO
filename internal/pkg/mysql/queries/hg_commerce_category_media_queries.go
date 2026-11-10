package SQLQueriesPackage

// 管理员类目图片按 (user_id,status,id) 索引计数及有界列举；绑定按 URL 哈希与上传者点查，禁止跨管理员使用。
const (
	// 上传预留先插入并锁定本人配额行，再计数和预留，避免多 Pod 并发突破 20 张上限。
	HGCommerceCategoryMediaQuotaSeedSQL     = `INSERT IGNORE INTO commerce_category_media_quota(user_id) VALUES(?)`
	HGCommerceCategoryMediaQuotaLockSQL     = `SELECT user_id FROM commerce_category_media_quota WHERE user_id=? FOR UPDATE`
	HGCommerceCategoryMediaCountSQL         = `SELECT COUNT(*) FROM commerce_category_media WHERE user_id=? AND status IN (0,1,3)`
	HGCommerceCategoryMediaReserveSQL       = `INSERT INTO commerce_category_media(user_id,image_url,storage_key) VALUES(?,?,?)`
	HGCommerceCategoryMediaReadySQL         = `UPDATE commerce_category_media SET status=1 WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=0`
	HGCommerceCategoryMediaRemovePendingSQL = `DELETE FROM commerce_category_media WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=0`
	HGCommerceCategoryMediaBindSQL          = `UPDATE commerce_category_media SET category_id=?,status=2 WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=1`
	HGCommerceCategoryMediaMoveSQL          = `UPDATE commerce_category_media SET category_id=? WHERE category_id=?`
	HGCommerceCategoryMediaUnbindSQL        = `UPDATE commerce_category_media SET category_id=NULL,status=1 WHERE category_id=? AND image_url=? AND status=2`
	HGCommerceCategoryMediaUnusedSQL        = `SELECT image_url FROM commerce_category_media WHERE user_id=? AND status IN (1,3) ORDER BY id DESC LIMIT 20`
	HGCommerceCategoryMediaClaimDeleteSQL   = `UPDATE commerce_category_media SET status=3 WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=1`
	HGCommerceCategoryMediaKeySQL           = `SELECT storage_key FROM commerce_category_media WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=3`
	HGCommerceCategoryMediaUndoDeleteSQL    = `UPDATE commerce_category_media SET status=1 WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=3`
	HGCommerceCategoryMediaFinishDeleteSQL  = `DELETE FROM commerce_category_media WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=3`
)

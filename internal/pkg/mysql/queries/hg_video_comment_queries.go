package SQLQueriesPackage

const (
	// SelectCommentableSubmissionSQL 分别命中稿件和视频唯一键，将页面 video_id 解析为权威 submission_id。
	SelectCommentableSubmissionSQL = `SELECT submission_id
FROM video_submissions
WHERE submission_id = ? AND status IN ('reviewing', 'published') AND visibility = 'public' AND close_comment = 0
UNION ALL
SELECT vs.submission_id
FROM video_files vf
INNER JOIN video_submissions vs ON vs.submission_id = vf.submission_id
WHERE vf.video_id = ? AND vs.status IN ('reviewing', 'published') AND vs.visibility = 'public' AND vs.close_comment = 0
LIMIT 1`

	// SelectVideoCommentParentForUpdateSQL 先锁直接父评论；nullable root_comment_id 用于区分直接根回复和嵌套回复。
	SelectVideoCommentParentForUpdateSQL = `SELECT comment_id, root_comment_id, user_id FROM video_comments WHERE comment_id = ? AND submission_id = ? AND is_deleted = 0 LIMIT 1 FOR UPDATE`
	// SelectVideoCommentRootForUpdateSQL 在嵌套回复中于父评论之后锁根评论，并再次确认根仍可见。
	SelectVideoCommentRootForUpdateSQL = `SELECT comment_id FROM video_comments WHERE comment_id = ? AND submission_id = ? AND root_comment_id IS NULL AND is_deleted = 0 LIMIT 1 FOR UPDATE`

	// InsertVideoCommentSQL 在短事务内写入顶级或回复评论；关系字段只接受服务端派生值。
	InsertVideoCommentSQL = `INSERT INTO video_comments (comment_id, submission_id, user_id, request_id, root_comment_id, parent_comment_id, reply_to_user_id, content, image_urls) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS JSON))`

	// IncrementVideoCommentStatShardSQL 只更新 submission 的 32 个固定分片之一，避免单统计行热点。
	IncrementVideoCommentStatShardSQL  = `INSERT INTO video_comment_stat_shards (submission_id, shard_id, comment_count) VALUES (?, ?, 1) ON DUPLICATE KEY UPDATE comment_count = comment_count + 1`
	DecrementVideoCommentStatShardSQL  = `UPDATE video_comment_stat_shards SET comment_count = IF(comment_count > 0, comment_count - 1, 0) WHERE submission_id = ? AND shard_id = ?`
	IncrementVideoCommentReplyCountSQL = `UPDATE video_comments SET reply_count = reply_count + 1 WHERE comment_id = ? AND is_deleted = 0`
	DecrementVideoCommentReplyCountSQL = `UPDATE video_comments SET reply_count = IF(reply_count > 0, reply_count - 1, 0) WHERE comment_id = ?`

	// 评论投影一次 LEFT JOIN 当前用户关系，避免列表中按评论执行 N+1 反应查询。
	SelectVideoCommentByCommentIDSQL = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.root_comment_id, vc.parent_comment_id, vc.reply_to_user_id, vc.like_count, vc.dislike_count, vc.reply_count, COALESCE(vcr.reaction, 'none'), COALESCE(JSON_UNQUOTE(vc.image_urls), '[]'), vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id LEFT JOIN video_comment_reactions vcr ON vcr.comment_id = vc.comment_id AND vcr.user_id = ? WHERE vc.comment_id = ? AND vc.is_deleted = 0 LIMIT 1`
	SelectVideoCommentByRequestIDSQL = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.root_comment_id, vc.parent_comment_id, vc.reply_to_user_id, vc.like_count, vc.dislike_count, vc.reply_count, COALESCE(vcr.reaction, 'none'), COALESCE(JSON_UNQUOTE(vc.image_urls), '[]'), vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id LEFT JOIN video_comment_reactions vcr ON vcr.comment_id = vc.comment_id AND vcr.user_id = ? WHERE vc.user_id = ? AND vc.request_id = ? LIMIT 1`

	// latest/hot 顶级评论查询保留完整复合游标，不使用 OFFSET 深分页。
	ListVideoCommentsLatestFirstSQL    = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.root_comment_id, vc.parent_comment_id, vc.reply_to_user_id, vc.like_count, vc.dislike_count, vc.reply_count, COALESCE(vcr.reaction, 'none'), COALESCE(JSON_UNQUOTE(vc.image_urls), '[]'), vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id LEFT JOIN video_comment_reactions vcr ON vcr.comment_id = vc.comment_id AND vcr.user_id = ? WHERE vc.submission_id = ? AND vc.is_deleted = 0 AND vc.root_comment_id IS NULL ORDER BY vc.created_at DESC, vc.id DESC LIMIT ?`
	ListVideoCommentsLatestByCursorSQL = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.root_comment_id, vc.parent_comment_id, vc.reply_to_user_id, vc.like_count, vc.dislike_count, vc.reply_count, COALESCE(vcr.reaction, 'none'), COALESCE(JSON_UNQUOTE(vc.image_urls), '[]'), vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id LEFT JOIN video_comment_reactions vcr ON vcr.comment_id = vc.comment_id AND vcr.user_id = ? WHERE vc.submission_id = ? AND vc.is_deleted = 0 AND vc.root_comment_id IS NULL AND (vc.created_at, vc.id) < (?, ?) ORDER BY vc.created_at DESC, vc.id DESC LIMIT ?`
	ListVideoCommentsHotFirstSQL       = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.root_comment_id, vc.parent_comment_id, vc.reply_to_user_id, vc.like_count, vc.dislike_count, vc.reply_count, COALESCE(vcr.reaction, 'none'), COALESCE(JSON_UNQUOTE(vc.image_urls), '[]'), vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id LEFT JOIN video_comment_reactions vcr ON vcr.comment_id = vc.comment_id AND vcr.user_id = ? WHERE vc.submission_id = ? AND vc.is_deleted = 0 AND vc.root_comment_id IS NULL ORDER BY vc.like_count DESC, vc.reply_count DESC, vc.created_at DESC, vc.id DESC LIMIT ?`
	ListVideoCommentsHotByCursorSQL    = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.root_comment_id, vc.parent_comment_id, vc.reply_to_user_id, vc.like_count, vc.dislike_count, vc.reply_count, COALESCE(vcr.reaction, 'none'), COALESCE(JSON_UNQUOTE(vc.image_urls), '[]'), vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id LEFT JOIN video_comment_reactions vcr ON vcr.comment_id = vc.comment_id AND vcr.user_id = ? WHERE vc.submission_id = ? AND vc.is_deleted = 0 AND vc.root_comment_id IS NULL AND (vc.like_count, vc.reply_count, vc.created_at, vc.id) < (?, ?, ?, ?) ORDER BY vc.like_count DESC, vc.reply_count DESC, vc.created_at DESC, vc.id DESC LIMIT ?`

	// SelectVideoCommentTotalCountSQL 最多聚合 32 个固定分片，不对评论热表执行实时 COUNT。
	SelectVideoCommentTotalCountSQL = `SELECT COALESCE(SUM(comment_count), 0) FROM video_comment_stat_shards WHERE submission_id = ?`

	// 回复分页命中 idx_video_comments_replies，并按 (created_at,id) 正序稳定前进。
	SelectVideoCommentRootListMetadataSQL = `SELECT submission_id, reply_count FROM video_comments WHERE comment_id = ? AND root_comment_id IS NULL AND is_deleted = 0 LIMIT 1`
	ListVideoCommentRepliesFirstSQL       = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.root_comment_id, vc.parent_comment_id, vc.reply_to_user_id, vc.like_count, vc.dislike_count, vc.reply_count, COALESCE(vcr.reaction, 'none'), COALESCE(JSON_UNQUOTE(vc.image_urls), '[]'), vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id LEFT JOIN video_comment_reactions vcr ON vcr.comment_id = vc.comment_id AND vcr.user_id = ? WHERE vc.submission_id = ? AND vc.root_comment_id = ? AND vc.is_deleted = 0 ORDER BY vc.created_at ASC, vc.id ASC LIMIT ?`
	ListVideoCommentRepliesByCursorSQL    = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.root_comment_id, vc.parent_comment_id, vc.reply_to_user_id, vc.like_count, vc.dislike_count, vc.reply_count, COALESCE(vcr.reaction, 'none'), COALESCE(JSON_UNQUOTE(vc.image_urls), '[]'), vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id LEFT JOIN video_comment_reactions vcr ON vcr.comment_id = vc.comment_id AND vcr.user_id = ? WHERE vc.submission_id = ? AND vc.root_comment_id = ? AND vc.is_deleted = 0 AND (vc.created_at, vc.id) > (?, ?) ORDER BY vc.created_at ASC, vc.id ASC LIMIT ?`

	SelectVideoCommentReactionTargetSQL        = `SELECT comment_id FROM video_comments WHERE comment_id = ? AND is_deleted = 0 LIMIT 1 FOR SHARE`
	SelectVideoCommentReactionBackfillReadySQL = `SELECT completed FROM video_comment_reaction_backfill_state WHERE job_name = 'reaction_shards_v1' LIMIT 1 FOR SHARE`
	EnsureVideoCommentReactionSQL              = `INSERT INTO video_comment_reactions (comment_id, user_id, reaction) VALUES (?, ?, 'none') ON DUPLICATE KEY UPDATE comment_id = VALUES(comment_id)`
	SelectVideoCommentReactionForUpdateSQL     = `SELECT reaction FROM video_comment_reactions WHERE comment_id = ? AND user_id = ? LIMIT 1 FOR UPDATE`
	UpsertVideoCommentReactionSQL              = `INSERT INTO video_comment_reactions (comment_id, user_id, reaction) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE reaction = VALUES(reaction), updated_at = CURRENT_TIMESTAMP(6)`
	DeleteVideoCommentReactionSQL              = `DELETE FROM video_comment_reactions WHERE comment_id = ? AND user_id = ?`
	UpdateVideoCommentReactionShardSQL         = `INSERT INTO video_comment_reaction_shards (comment_id, shard_id, like_count, dislike_count) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE like_count = GREATEST(like_count + VALUES(like_count), 0), dislike_count = GREATEST(dislike_count + VALUES(dislike_count), 0)`
	MarkVideoCommentReactionDirtySQL           = `INSERT INTO video_comment_reaction_dirty (comment_id, revision) VALUES (?, 1) ON DUPLICATE KEY UPDATE revision = revision + 1`
	SelectVideoCommentReactionShardTotalsSQL   = `SELECT COALESCE(SUM(like_count), 0), COALESCE(SUM(dislike_count), 0) FROM video_comment_reaction_shards WHERE comment_id = ?`

	SelectVideoCommentDeleteTargetForUpdateSQL = `SELECT submission_id, root_comment_id, reply_count FROM video_comments WHERE comment_id = ? AND user_id = ? AND is_deleted = 0 LIMIT 1 FOR UPDATE`
	SoftDeleteVideoCommentSQL                  = `UPDATE video_comments SET is_deleted = 1, deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE comment_id = ? AND user_id = ? AND is_deleted = 0`

	EnsureVideoCommentImageQuotaSQL                       = `INSERT INTO video_comment_image_quotas (user_id, used_bytes, asset_count) VALUES (?, 0, 0) ON DUPLICATE KEY UPDATE user_id = VALUES(user_id)`
	ReserveVideoCommentImageQuotaSQL                      = `UPDATE video_comment_image_quotas SET used_bytes = used_bytes + ?, asset_count = asset_count + 1 WHERE user_id = ? AND used_bytes + ? <= ?`
	ReleaseVideoCommentImageQuotaSQL                      = `UPDATE video_comment_image_quotas SET used_bytes = IF(used_bytes >= ?, used_bytes - ?, 0), asset_count = IF(asset_count > 0, asset_count - 1, 0) WHERE user_id = ?`
	InsertVideoCommentImageAssetSQL                       = `INSERT INTO video_comment_images (image_id, user_id, storage_key, image_url, size_bytes, content_type, status) VALUES (?, ?, ?, ?, ?, ?, 'pending')`
	ScheduleVideoCommentImageCleanupSQL                   = `UPDATE video_comment_images SET status = 'delete_pending', delete_after = CURRENT_TIMESTAMP(6) WHERE image_id = ? AND user_id = ? AND status = 'pending'`
	SelectVideoCommentImageForAttachSQL                   = `SELECT image_id FROM video_comment_images WHERE image_url_hash = UNHEX(SHA2(?, 256)) AND image_url = ? AND user_id = ? AND status = 'pending' LIMIT 1 FOR UPDATE`
	AttachVideoCommentImageSQL                            = `UPDATE video_comment_images SET status = 'attached', comment_id = ?, attached_at = CURRENT_TIMESTAMP(6) WHERE image_id = ? AND user_id = ? AND status = 'pending'`
	MarkVideoCommentImagesDeletePendingSQL                = `UPDATE video_comment_images SET status = 'delete_pending', delete_after = CURRENT_TIMESTAMP(6) WHERE comment_id = ? AND status = 'attached'`
	ListVideoCommentReactionDirtySQL                      = `SELECT comment_id, revision FROM video_comment_reaction_dirty ORDER BY updated_at ASC, comment_id ASC LIMIT ?`
	ProjectVideoCommentReactionCountsSQL                  = `UPDATE video_comments vc SET like_count = (SELECT COALESCE(SUM(s.like_count), 0) FROM video_comment_reaction_shards s WHERE s.comment_id = vc.comment_id), dislike_count = (SELECT COALESCE(SUM(s.dislike_count), 0) FROM video_comment_reaction_shards s WHERE s.comment_id = vc.comment_id) WHERE vc.comment_id = ?`
	DeleteVideoCommentReactionDirtySQL                    = `DELETE FROM video_comment_reaction_dirty WHERE comment_id = ? AND revision = ?`
	RequeueVideoCommentReactionDirtySQL                   = `UPDATE video_comment_reaction_dirty SET updated_at = CURRENT_TIMESTAMP(6) WHERE comment_id = ? AND revision <> ?`
	ListPendingVideoCommentImageCleanupForUpdateSQL       = `SELECT image_id, user_id, storage_key, size_bytes FROM video_comment_images WHERE status = 'pending' AND created_at < ? ORDER BY created_at ASC, id ASC LIMIT ? FOR UPDATE SKIP LOCKED`
	ListDeletePendingVideoCommentImageCleanupForUpdateSQL = `SELECT image_id, user_id, storage_key, size_bytes FROM video_comment_images WHERE status = 'delete_pending' AND delete_after <= CURRENT_TIMESTAMP(6) ORDER BY delete_after ASC, id ASC LIMIT ? FOR UPDATE SKIP LOCKED`
	ListExpiredVideoCommentImageCleanupForUpdateSQL       = `SELECT image_id, user_id, storage_key, size_bytes FROM video_comment_images WHERE status = 'deleting' AND cleanup_lease_until <= CURRENT_TIMESTAMP(6) ORDER BY cleanup_lease_until ASC, id ASC LIMIT ? FOR UPDATE SKIP LOCKED`
	MarkVideoCommentImageDeletingSQL                      = `UPDATE video_comment_images SET status = 'deleting', cleanup_token = ?, cleanup_lease_until = ? WHERE image_id = ? AND (status IN ('pending', 'delete_pending') OR (status = 'deleting' AND cleanup_lease_until <= CURRENT_TIMESTAMP(6)))`
	DeleteVideoCommentImageAssetSQL                       = `DELETE FROM video_comment_images WHERE image_id = ? AND status = 'deleting' AND cleanup_token = ?`
	ReleaseVideoCommentImageCleanupSQL                    = `UPDATE video_comment_images SET status = 'delete_pending', delete_after = CURRENT_TIMESTAMP(6), cleanup_token = NULL, cleanup_lease_until = NULL, cleanup_retry_count = cleanup_retry_count + 1 WHERE image_id = ? AND status = 'deleting' AND cleanup_token = ?`
)

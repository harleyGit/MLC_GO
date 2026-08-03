package SQLQueriesPackage

const (
	// SelectCommentableSubmissionSQL 命中 submission_id 唯一键点查，仅允许已发布、公开且未关闭评论的稿件。
	SelectCommentableSubmissionSQL = `SELECT submission_id FROM video_submissions WHERE submission_id = ? AND status = 'published' AND visibility = 'public' AND close_comment = 0 LIMIT 1`

	// InsertVideoCommentSQL 在短事务内同步写入顶级评论；(user_id, request_id) 唯一键承担并发幂等。
	InsertVideoCommentSQL = `INSERT INTO video_comments (comment_id, submission_id, user_id, request_id, content) VALUES (?, ?, ?, ?, ?)`

	// SelectVideoCommentByCommentIDSQL 按 comment_id 唯一键读取新建评论及用户展示信息。
	SelectVideoCommentByCommentIDSQL = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.like_count, vc.reply_count, vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id WHERE vc.comment_id = ? AND vc.is_deleted = 0 LIMIT 1`

	// SelectVideoCommentByRequestIDSQL 在唯一键冲突时读取同一用户、同一 request_id 的权威结果。
	SelectVideoCommentByRequestIDSQL = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.like_count, vc.reply_count, vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id WHERE vc.user_id = ? AND vc.request_id = ? LIMIT 1`

	// ListVideoCommentsLatestFirstSQL 命中顶级评论 latest 复合索引，多取一条判断下一页，不使用 OFFSET/COUNT。
	ListVideoCommentsLatestFirstSQL = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.like_count, vc.reply_count, vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id WHERE vc.submission_id = ? AND vc.is_deleted = 0 AND vc.root_comment_id IS NULL ORDER BY vc.created_at DESC, vc.id DESC LIMIT ?`

	// ListVideoCommentsLatestByCursorSQL 使用 (created_at,id) 稳定复合游标，避免深分页扫描。
	ListVideoCommentsLatestByCursorSQL = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.like_count, vc.reply_count, vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id WHERE vc.submission_id = ? AND vc.is_deleted = 0 AND vc.root_comment_id IS NULL AND (vc.created_at, vc.id) < (?, ?) ORDER BY vc.created_at DESC, vc.id DESC LIMIT ?`

	// ListVideoCommentsHotFirstSQL 命中顶级评论 hot 复合索引，多取一条判断下一页。
	ListVideoCommentsHotFirstSQL = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.like_count, vc.reply_count, vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id WHERE vc.submission_id = ? AND vc.is_deleted = 0 AND vc.root_comment_id IS NULL ORDER BY vc.like_count DESC, vc.reply_count DESC, vc.created_at DESC, vc.id DESC LIMIT ?`

	// ListVideoCommentsHotByCursorSQL 使用完整 hot 排序元组作为 keyset，不使用 OFFSET/COUNT。
	ListVideoCommentsHotByCursorSQL = `SELECT vc.id, vc.comment_id, vc.submission_id, vc.user_id, COALESCE(NULLIF(u.nickname, ''), NULLIF(u.user_name, ''), vc.user_id), COALESCE(u.avatar_url, ''), vc.content, vc.like_count, vc.reply_count, vc.created_at FROM video_comments vc INNER JOIN users u ON u.user_id = vc.user_id WHERE vc.submission_id = ? AND vc.is_deleted = 0 AND vc.root_comment_id IS NULL AND (vc.like_count, vc.reply_count, vc.created_at, vc.id) < (?, ?, ?, ?) ORDER BY vc.like_count DESC, vc.reply_count DESC, vc.created_at DESC, vc.id DESC LIMIT ?`

	// SoftDeleteVideoCommentSQL 由 comment_id 唯一键定位，并以 user_id 限定作者权限和更新范围。
	SoftDeleteVideoCommentSQL = `UPDATE video_comments SET is_deleted = 1, deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE comment_id = ? AND user_id = ? AND is_deleted = 0`
)

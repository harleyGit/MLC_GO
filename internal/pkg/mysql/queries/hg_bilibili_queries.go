package SQLQueriesPackage

const (
	// SelectBilibiliAuthorProfileSQL 仅按 users.user_id 唯一索引读取公开字段，禁止返回账号安全信息。
	SelectBilibiliAuthorProfileSQL = `SELECT user_id, COALESCE(user_name, ''), COALESCE(nickname, ''), COALESCE(signature, ''), gender, COALESCE(avatar_url, ''), created_at
		FROM users WHERE user_id = ? LIMIT 1`

	// SelectBilibiliAuthorVideosFirstSQL 命中 idx_bilibili_author_videos，使用 publish_time + submission_id 稳定排序。
	SelectBilibiliAuthorVideosFirstSQL = `SELECT vs.submission_id, COALESCE(vf.video_id, ''), vs.user_id, vs.title,
		COALESCE(NULLIF(vs.cover_url, ''), vf.cover_url, ''), vs.category, COALESCE(vs.description, ''),
		COALESCE(vf.duration, 0), COALESCE(vf.file_path, ''), vs.publish_time
		FROM video_submissions vs
		LEFT JOIN video_files vf ON vf.submission_id = vs.submission_id AND vf.part_number = 1
		WHERE vs.user_id = ? AND vs.status = 'published' AND vs.visibility = 'public'
		AND vs.hide_from_profile = 0 AND vs.publish_time IS NOT NULL
		ORDER BY vs.publish_time DESC, vs.submission_id DESC LIMIT ?`

	// SelectBilibiliAuthorVideosByCursorSQL 使用复合 keyset 游标，避免 OFFSET 随页码增长退化。
	SelectBilibiliAuthorVideosByCursorSQL = `SELECT vs.submission_id, COALESCE(vf.video_id, ''), vs.user_id, vs.title,
		COALESCE(NULLIF(vs.cover_url, ''), vf.cover_url, ''), vs.category, COALESCE(vs.description, ''),
		COALESCE(vf.duration, 0), COALESCE(vf.file_path, ''), vs.publish_time
		FROM video_submissions vs
		LEFT JOIN video_files vf ON vf.submission_id = vs.submission_id AND vf.part_number = 1
		WHERE vs.user_id = ? AND vs.status = 'published' AND vs.visibility = 'public'
		AND vs.hide_from_profile = 0 AND vs.publish_time IS NOT NULL
		AND (vs.publish_time < ? OR (vs.publish_time = ? AND vs.submission_id < ?))
		ORDER BY vs.publish_time DESC, vs.submission_id DESC LIMIT ?`

	// CountBilibiliAuthorVideosSQL 由 idx_bilibili_author_videos 覆盖过滤条件，不扫描其他作者稿件。
	CountBilibiliAuthorVideosSQL = `SELECT COUNT(*) FROM video_submissions
		WHERE user_id = ? AND status = 'published' AND visibility = 'public' AND hide_from_profile = 0 AND publish_time IS NOT NULL`
	// CountBilibiliAuthorFollowingSQL 命中 idx_bilibili_following_count，仅统计当前作者有效关注关系。
	CountBilibiliAuthorFollowingSQL = `SELECT COUNT(*) FROM user_follow_relations WHERE follower_id = ? AND active = 1`
	// SumBilibiliAuthorFollowersSQL 按主键 user_id 前缀最多聚合固定数量统计分片，作为 Redis 投影失效时的有界回源。
	SumBilibiliAuthorFollowersSQL = `SELECT COALESCE(SUM(follower_count), 0) FROM user_follow_stat_shards WHERE user_id = ?`
)

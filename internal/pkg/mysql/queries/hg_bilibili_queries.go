package SQLQueriesPackage

const (
	// SelectBilibiliAuthorProfileSQL 仅按 users.user_id 唯一索引读取公开字段，禁止返回账号安全信息。
	SelectBilibiliAuthorProfileSQL = `SELECT user_id, COALESCE(user_name, ''), COALESCE(nickname, ''), COALESCE(signature, ''), gender, COALESCE(avatar_url, ''), created_at
		FROM users WHERE user_id = ? LIMIT 1`

	// SelectBilibiliAuthorVideosFirstSQL 按作者索引限定公开稿件；当前审核流把 reviewing 作为可播放状态，时间字段兼容历史空 publish_time。
	SelectBilibiliAuthorVideosFirstSQL = `SELECT vs.submission_id, COALESCE(vf.video_id, ''), vs.user_id, vs.title,
		COALESCE(NULLIF(vs.cover_url, ''), vf.cover_url, ''), vs.category, COALESCE(vs.description, ''),
		COALESCE(vf.duration, 0), COALESCE(vf.file_path, ''), COALESCE(vs.publish_time, vs.submit_time, vs.created_at) AS profile_publish_time
		FROM video_submissions vs
		LEFT JOIN video_files vf ON vf.submission_id = vs.submission_id AND vf.part_number = 1
		WHERE vs.user_id = ? AND vs.status IN ('reviewing', 'published') AND vs.visibility = 'public'
		AND vs.hide_from_profile = 0
		ORDER BY profile_publish_time DESC, vs.submission_id DESC LIMIT ?`

	// SelectBilibiliAuthorVideosByCursorSQL 使用展示时间 + submission_id 复合游标，避免 OFFSET 深分页并兼容历史时间字段。
	SelectBilibiliAuthorVideosByCursorSQL = `SELECT vs.submission_id, COALESCE(vf.video_id, ''), vs.user_id, vs.title,
		COALESCE(NULLIF(vs.cover_url, ''), vf.cover_url, ''), vs.category, COALESCE(vs.description, ''),
		COALESCE(vf.duration, 0), COALESCE(vf.file_path, ''), COALESCE(vs.publish_time, vs.submit_time, vs.created_at) AS profile_publish_time
		FROM video_submissions vs
		LEFT JOIN video_files vf ON vf.submission_id = vs.submission_id AND vf.part_number = 1
		WHERE vs.user_id = ? AND vs.status IN ('reviewing', 'published') AND vs.visibility = 'public'
		AND vs.hide_from_profile = 0
		AND (COALESCE(vs.publish_time, vs.submit_time, vs.created_at) < ? OR (COALESCE(vs.publish_time, vs.submit_time, vs.created_at) = ? AND vs.submission_id < ?))
		ORDER BY profile_publish_time DESC, vs.submission_id DESC LIMIT ?`

	// CountBilibiliAuthorVideosSQL 由 idx_bilibili_author_videos 按 user_id 前缀限定，不扫描其他作者稿件。
	CountBilibiliAuthorVideosSQL = `SELECT COUNT(*) FROM video_submissions
		WHERE user_id = ? AND status IN ('reviewing', 'published') AND visibility = 'public' AND hide_from_profile = 0`
	// CountBilibiliAuthorFollowingSQL 命中 idx_bilibili_following_count，仅统计当前作者有效关注关系。
	CountBilibiliAuthorFollowingSQL = `SELECT COUNT(*) FROM user_follow_relations WHERE follower_id = ? AND active = 1`
	// SumBilibiliAuthorFollowersSQL 按主键 user_id 前缀最多聚合固定数量统计分片，作为 Redis 投影失效时的有界回源。
	SumBilibiliAuthorFollowersSQL = `SELECT COALESCE(SUM(follower_count), 0) FROM user_follow_stat_shards WHERE user_id = ?`
)

package SQLQueriesPackage

const (
	// SelectVideoRecommendCardsPrefixSQL 通过 video_submissions.uk_submission_id 批量点查候选卡片；调用方必须限制 IN 数量。
	// 查询只接受 Redis 已召回的少量业务 ID，并再次限制 published/public，避免审核中或已下架内容曝光。
	SelectVideoRecommendCardsPrefixSQL = `SELECT vs.submission_id, COALESCE(vf.video_id, ''), vs.user_id, vs.title,
		COALESCE(NULLIF(vs.cover_url, ''), vf.cover_url, ''), vs.category, COALESCE(vs.description, ''),
		COALESCE(vf.duration, 0), COALESCE(vf.file_path, ''), COALESCE(vs.publish_time, vs.created_at)
		FROM video_submissions vs
		LEFT JOIN video_files vf ON vf.submission_id = vs.submission_id AND vf.part_number = 1
		WHERE vs.status = 'published' AND vs.visibility = 'public' AND vs.submission_id IN (`
	SelectVideoRecommendCardsSuffixSQL = `)`
)

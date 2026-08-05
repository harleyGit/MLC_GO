package SQLQueriesPackage

const (
	// SelectDanmakuVideoTargetSQL 通过唯一 video_id 读取页面可见且允许弹幕的视频，避免热表写入前扫描稿件。
	// reviewing/published 与视频列表、详情和评论模块保持一致；visibility 和 close_danmaku 继续阻止私有或已关闭弹幕的视频进入实时房间。
	SelectDanmakuVideoTargetSQL = `SELECT vf.video_id, vs.submission_id
FROM video_files vf
INNER JOIN video_submissions vs ON vs.submission_id = vf.submission_id
WHERE vf.video_id = ? AND vs.status IN ('reviewing', 'published') AND vs.visibility = 'public' AND vs.close_danmaku = 0
LIMIT 1`

	// InsertVideoDanmakuSQL 写入权威弹幕；(user_id,request_id) 唯一键提供用户维度幂等。
	InsertVideoDanmakuSQL = `INSERT INTO video_danmaku
(danmaku_id, submission_id, video_id, user_id, request_id, progress_ms, content, mode, color, font_size)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	SelectVideoDanmakuByRequestIDSQL = `SELECT id, danmaku_id, submission_id, video_id, user_id, progress_ms, content, mode, color, font_size, created_at
FROM video_danmaku WHERE user_id = ? AND request_id = ? LIMIT 1`
	// SelectVideoDanmakuByPrimaryIDSQL 在 INSERT 后使用 LastInsertId 命中聚簇主键，避免亿级热表再次走 danmaku_id 二级索引。
	SelectVideoDanmakuByPrimaryIDSQL = `SELECT id, danmaku_id, submission_id, video_id, user_id, progress_ms, content, mode, color, font_size, created_at
	FROM video_danmaku WHERE id = ? LIMIT 1`

	// IncrementVideoDanmakuStatShardSQL 将同一视频计数分散到 64 行，避免热门视频单行更新锁竞争。
	IncrementVideoDanmakuStatShardSQL = `INSERT INTO video_danmaku_stat_shards (video_id, shard_id, danmaku_count)
VALUES (?, ?, 1) ON DUPLICATE KEY UPDATE danmaku_count = danmaku_count + 1`
	SelectVideoDanmakuTotalSQL = `SELECT COALESCE(SUM(danmaku_count), 0) FROM video_danmaku_stat_shards WHERE video_id = ?`

	// 时间窗列表命中 idx_video_danmaku_timeline，使用 (progress_ms,id) keyset，不使用 OFFSET。
	ListVideoDanmakuFirstSQL = `SELECT id, danmaku_id, submission_id, video_id, user_id, progress_ms, content, mode, color, font_size, created_at
FROM video_danmaku
WHERE video_id = ? AND status = 'active' AND progress_ms >= ? AND progress_ms < ?
ORDER BY progress_ms ASC, id ASC LIMIT ?`
	ListVideoDanmakuByCursorSQL = `SELECT id, danmaku_id, submission_id, video_id, user_id, progress_ms, content, mode, color, font_size, created_at
FROM video_danmaku
WHERE video_id = ? AND status = 'active' AND progress_ms >= ? AND progress_ms < ?
  AND (progress_ms, id) > (?, ?)
ORDER BY progress_ms ASC, id ASC LIMIT ?`
)

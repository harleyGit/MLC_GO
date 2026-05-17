USE HG_MLC_DB;

-- ============================================
-- 000006: 回滚视频投稿业务表
-- 按外键依赖逆序删除，避免约束冲突。
-- ============================================

DROP TABLE IF EXISTS `video_subtitles`;
DROP TABLE IF EXISTS `video_chapters`;
DROP TABLE IF EXISTS `video_commercial_promotion`;
DROP TABLE IF EXISTS `video_scheduled_publish`;
DROP TABLE IF EXISTS `video_tags`;
DROP TABLE IF EXISTS `video_files`;
DROP TABLE IF EXISTS `video_submissions`;

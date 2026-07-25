USE HG_MLC_DB;

-- 回滚顺序先移除 video_tags 上新增的查询索引，再删除独立标签目录；历史视频标签数据不会被删除。
ALTER TABLE `video_tags` DROP INDEX `idx_tag_name_video_id`;
DROP TABLE IF EXISTS `bilibili_douga_tags`;

USE HG_MLC_DB;

ALTER TABLE `user_follow_relations` DROP INDEX `idx_bilibili_following_count`;
ALTER TABLE `video_submissions` DROP INDEX `idx_bilibili_author_videos`;

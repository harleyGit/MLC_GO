USE HG_MLC_DB;

-- 作者公开视频按 user_id 定位，固定公开状态后按 publish_time + submission_id 游标倒序读取。
-- 生产大表执行前应根据 MySQL 版本确认 Online DDL 能力，并在低峰期观察 metadata lock。
ALTER TABLE `video_submissions`
    ADD INDEX `idx_bilibili_author_videos`
    (`user_id`, `status`, `visibility`, `hide_from_profile`, `publish_time` DESC, `submission_id` DESC);

-- 作者关注数查询固定 follower_id + active，避免在关系大表上扫描无效历史关系。
ALTER TABLE `user_follow_relations`
    ADD INDEX `idx_bilibili_following_count` (`follower_id`, `active`, `followee_id`);

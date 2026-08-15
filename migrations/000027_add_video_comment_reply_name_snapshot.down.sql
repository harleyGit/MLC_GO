DROP TABLE IF EXISTS `video_comment_reply_dirty`;
DROP TABLE IF EXISTS `video_comment_reply_shards`;

ALTER TABLE `video_comments`
    DROP COLUMN `reply_to_user_name`;

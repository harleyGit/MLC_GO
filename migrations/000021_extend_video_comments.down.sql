DROP TABLE IF EXISTS `video_comment_stat_shards`;
DROP TABLE IF EXISTS `video_comment_reactions`;

ALTER TABLE `video_comments`
    DROP KEY `idx_video_comments_replies`,
    DROP COLUMN `image_urls`,
    DROP COLUMN `dislike_count`;

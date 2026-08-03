-- 扩展评论热表：点踩数参与展示但不进入 hot 排序；图片只保存受控上传 URL 数组。
ALTER TABLE `video_comments`
    ADD COLUMN `dislike_count` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '点踩数' AFTER `like_count`,
    ADD COLUMN `image_urls` JSON NULL COMMENT '评论图片URL数组，最多3个' AFTER `content`,
    ADD KEY `idx_video_comments_replies` (`submission_id`, `root_comment_id`, `is_deleted`, `created_at`, `id`);

-- 先回填空数组再收紧 NOT NULL，兼容迁移前已有评论。
UPDATE `video_comments` SET `image_urls` = JSON_ARRAY() WHERE `image_urls` IS NULL;
ALTER TABLE `video_comments` MODIFY COLUMN `image_urls` JSON NOT NULL COMMENT '评论图片URL数组，最多3个';

CREATE TABLE `video_comment_reactions` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `comment_id` VARCHAR(64) NOT NULL,
    `user_id` VARCHAR(255) NOT NULL,
    `reaction` ENUM('like', 'dislike') NOT NULL,
    `created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_video_comment_reactions_comment_user` (`comment_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频评论用户反应关系';

-- 每个 submission 最多 32 行，写入按 comment_id 稳定分片，运行期总数只聚合固定上限结果集。
CREATE TABLE `video_comment_stat_shards` (
    `submission_id` VARCHAR(64) NOT NULL,
    `shard_id` TINYINT UNSIGNED NOT NULL,
    `comment_count` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`submission_id`, `shard_id`),
    CONSTRAINT `chk_video_comment_stat_shard_id` CHECK (`shard_id` < 32)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频评论总数32分片';

INSERT INTO `video_comment_stat_shards` (`submission_id`, `shard_id`, `comment_count`)
SELECT `submission_id`, CRC32(`comment_id`) % 32, COUNT(*)
FROM `video_comments`
WHERE `is_deleted` = 0
GROUP BY `submission_id`, CRC32(`comment_id`) % 32;

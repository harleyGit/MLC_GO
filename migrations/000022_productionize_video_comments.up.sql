-- User relationship rows serialize only one user's state change; aggregate writes are spread over 32 rows per comment.
ALTER TABLE `video_comment_reactions`
    MODIFY COLUMN `reaction` ENUM('none', 'like', 'dislike') NOT NULL;

CREATE TABLE `video_comment_reaction_shards` (
    `comment_id` VARCHAR(64) NOT NULL,
    `shard_id` TINYINT UNSIGNED NOT NULL,
    `like_count` BIGINT NOT NULL DEFAULT 0,
    `dislike_count` BIGINT NOT NULL DEFAULT 0,
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`comment_id`, `shard_id`),
    CONSTRAINT `chk_video_comment_reaction_shard_id` CHECK (`shard_id` < 32),
    CONSTRAINT `chk_video_comment_reaction_shard_counts` CHECK (`like_count` >= 0 AND `dislike_count` >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频评论赞踩32分片权威计数';

INSERT INTO `video_comment_reaction_shards` (`comment_id`, `shard_id`, `like_count`, `dislike_count`)
SELECT `comment_id`, CRC32(`user_id`) % 32,
       SUM(`reaction` = 'like'), SUM(`reaction` = 'dislike')
FROM `video_comment_reactions`
GROUP BY `comment_id`, CRC32(`user_id`) % 32;

CREATE TABLE `video_comment_reaction_dirty` (
    `comment_id` VARCHAR(64) NOT NULL,
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`comment_id`),
    KEY `idx_video_comment_reaction_dirty_updated` (`updated_at`, `comment_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='待投影评论赞踩计数';

CREATE TABLE `video_comment_image_quotas` (
    `user_id` VARCHAR(255) NOT NULL,
    `used_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `asset_count` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频评论图片用户权威容量';

CREATE TABLE `video_comment_images` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `image_id` VARCHAR(64) NOT NULL,
    `user_id` VARCHAR(255) NOT NULL,
    `storage_key` VARCHAR(512) NOT NULL,
    `image_url` VARCHAR(1024) NOT NULL,
    `image_url_hash` BINARY(32) GENERATED ALWAYS AS (UNHEX(SHA2(`image_url`, 256))) STORED,
    `size_bytes` BIGINT UNSIGNED NOT NULL,
    `content_type` VARCHAR(64) NOT NULL,
    `status` ENUM('pending', 'attached', 'delete_pending', 'deleting') NOT NULL DEFAULT 'pending',
    `comment_id` VARCHAR(64) NULL,
    `created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    `attached_at` TIMESTAMP(6) NULL,
    `delete_after` TIMESTAMP(6) NULL,
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_video_comment_images_image_id` (`image_id`),
    UNIQUE KEY `uk_video_comment_images_storage_key` (`storage_key`),
    UNIQUE KEY `uk_video_comment_images_url_hash` (`image_url_hash`),
    KEY `idx_video_comment_images_owner` (`user_id`, `status`, `id`),
    KEY `idx_video_comment_images_comment` (`comment_id`, `status`, `id`),
    KEY `idx_video_comment_images_cleanup` (`status`, `delete_after`, `created_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频评论图片所有权和生命周期';

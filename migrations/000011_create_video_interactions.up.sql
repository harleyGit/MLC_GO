CREATE TABLE IF NOT EXISTS `video_interaction_inbox` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `event_id` VARCHAR(64) NOT NULL,
    `event_name` VARCHAR(64) NOT NULL,
    `event_key` VARCHAR(255) NOT NULL,
    `kafka_topic` VARCHAR(128) NOT NULL,
    `kafka_partition` INT NOT NULL,
    `kafka_offset` BIGINT NOT NULL,
    `payload` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event_id` (`event_id`),
    UNIQUE KEY `uk_kafka_delivery` (`kafka_topic`, `kafka_partition`, `kafka_offset`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Kafka 视频互动消费幂等表';

CREATE TABLE IF NOT EXISTS `video_user_interactions` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(255) NOT NULL,
    `submission_id` VARCHAR(64) NOT NULL,
    `interaction_type` VARCHAR(16) NOT NULL,
    `active` TINYINT(1) NOT NULL DEFAULT 0,
    `quantity` TINYINT UNSIGNED NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_submission_type` (`user_id`, `submission_id`, `interaction_type`),
    KEY `idx_submission_type_active` (`submission_id`, `interaction_type`, `active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户视频点赞投币收藏状态';

CREATE TABLE IF NOT EXISTS `video_interaction_stat_shards` (
    `submission_id` VARCHAR(64) NOT NULL,
    `shard_id` SMALLINT UNSIGNED NOT NULL,
    `like_count` BIGINT NOT NULL DEFAULT 0,
    `coin_count` BIGINT NOT NULL DEFAULT 0,
    `favorite_count` BIGINT NOT NULL DEFAULT 0,
    `share_count` BIGINT NOT NULL DEFAULT 0,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`submission_id`, `shard_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频互动分片统计，避免热门视频单行锁';

CREATE TABLE IF NOT EXISTS `video_share_records` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `event_id` VARCHAR(64) NOT NULL,
    `user_id` VARCHAR(255) NOT NULL,
    `submission_id` VARCHAR(64) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_share_event` (`event_id`),
    KEY `idx_submission_created` (`submission_id`, `created_at`),
    KEY `idx_user_created` (`user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频分享事件流水';

CREATE TABLE IF NOT EXISTS `user_follow_relations` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `follower_id` VARCHAR(255) NOT NULL,
    `followee_id` VARCHAR(255) NOT NULL,
    `active` TINYINT(1) NOT NULL DEFAULT 1,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_follower_followee` (`follower_id`, `followee_id`),
    KEY `idx_followee_active_follower` (`followee_id`, `active`, `follower_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户关注关系';

CREATE TABLE IF NOT EXISTS `user_follow_stat_shards` (
    `user_id` VARCHAR(255) NOT NULL,
    `shard_id` SMALLINT UNSIGNED NOT NULL,
    `follower_count` BIGINT NOT NULL DEFAULT 0,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`user_id`, `shard_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户粉丝分片统计';

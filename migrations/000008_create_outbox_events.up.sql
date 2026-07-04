USE HG_MLC_DB;

-- Outbox 本地消息表。
-- 业务写库和事件写入同一个 MySQL 事务，解决“主库成功但 Kafka 失败”导致的数据不一致。
CREATE TABLE IF NOT EXISTS `outbox_events` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `event_id` VARCHAR(64) NOT NULL COMMENT '事件唯一 ID，用于消费侧幂等',
    `event_name` VARCHAR(128) NOT NULL COMMENT '领域事件名，如 video.reviewed',
    `event_key` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Kafka key，同一实体事件按 key 保序',
    `topic` VARCHAR(128) NOT NULL COMMENT '目标 Kafka topic',
    `payload` JSON NOT NULL COMMENT 'EventEnvelope JSON 字节协议',
    `status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/published/dead',
    `retry_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '失败重试次数',
    `next_retry_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '下次允许投递时间',
    `last_error` VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '最后一次投递错误',
    `published_at` TIMESTAMP NULL COMMENT '成功投递 Kafka 时间',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event_id` (`event_id`),
    KEY `idx_status_next_id` (`status`, `next_retry_at`, `id`),
    KEY `idx_event_name` (`event_name`),
    KEY `idx_event_key` (`event_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Outbox 本地消息表';

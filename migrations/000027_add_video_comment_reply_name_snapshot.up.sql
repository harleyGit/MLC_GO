-- 仅新增 nullable 列，不在迁移中回填亿级评论热表；历史空快照由读取路径按页批量兼容。
-- 生产执行前需确认 MySQL 版本支持目标 Online DDL，并在低峰期监控 metadata lock 和复制延迟。
ALTER TABLE `video_comments`
    ADD COLUMN `reply_to_user_name` VARCHAR(255) NULL COMMENT '被回复用户昵称快照；顶级评论为空' AFTER `reply_to_user_id`;

-- 爆款根评论的回复写入分散到 256 行；首次访问旧根评论时由服务将现有 reply_count 放入 shard 0。
CREATE TABLE `video_comment_reply_shards` (
    `root_comment_id` VARCHAR(64) NOT NULL,
    `shard_id` SMALLINT UNSIGNED NOT NULL,
    `reply_count` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`root_comment_id`, `shard_id`),
    CONSTRAINT `chk_video_comment_reply_shard_id` CHECK (`shard_id` < 256)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频根评论回复数256分片权威计数';

CREATE TABLE `video_comment_reply_dirty` (
    `root_comment_id` VARCHAR(64) NOT NULL,
    `shard_id` SMALLINT UNSIGNED NOT NULL,
    `revision` BIGINT UNSIGNED NOT NULL DEFAULT 1,
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`root_comment_id`, `shard_id`),
    KEY `idx_video_comment_reply_dirty_updated` (`updated_at`, `root_comment_id`, `shard_id`),
    CONSTRAINT `chk_video_comment_reply_dirty_shard_id` CHECK (`shard_id` < 256)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='待投影根评论回复数';

-- revision 是 dirty 投影的 CAS 版本。移除 updated_at 的 ON UPDATE，避免热点评论每次 reaction 都自动刷新队列时间并长期饥饿。
ALTER TABLE `video_comment_reaction_dirty`
    ADD COLUMN `revision` BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER `comment_id`,
    MODIFY COLUMN `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6);

-- cleanup_token 是数据库 fencing token，cleanup_lease_until 允许 worker 崩溃后的 deleting 行重新领取。
-- 三个索引分别服务 pending、delete_pending 和过期 deleting 的单状态范围扫描，禁止恢复成多状态 OR 查询。
ALTER TABLE `video_comment_images`
    ADD COLUMN `cleanup_token` VARCHAR(64) NULL AFTER `delete_after`,
    ADD COLUMN `cleanup_lease_until` TIMESTAMP(6) NULL AFTER `cleanup_token`,
    ADD COLUMN `cleanup_retry_count` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `cleanup_lease_until`,
    ADD KEY `idx_video_comment_images_pending_cleanup` (`status`, `created_at`, `id`),
    ADD KEY `idx_video_comment_images_delete_cleanup` (`status`, `delete_after`, `id`),
    ADD KEY `idx_video_comment_images_deleting_lease` (`status`, `cleanup_lease_until`, `id`);

-- checkpoint 同时承担有界回填恢复游标和 reaction 在线写屏障；down 时有意保留，防止部分回填后重新升级丢失进度。
CREATE TABLE IF NOT EXISTS `video_comment_reaction_backfill_state` (
    `job_name` VARCHAR(64) NOT NULL,
    `cursor_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `completed` TINYINT(1) NOT NULL DEFAULT 0,
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`job_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频评论反应分片有界回填游标';

-- 已执行旧版 000022 全量回填的环境已有 shard 数据，直接标记完成，禁止再次增量累加。
-- 新环境 shard 为空时 cursor 保持 0、completed 保持 0，必须运行独立 backfill 命令后才能开放 reaction 写入。
-- INSERT IGNORE 保留 down/up 演练或部分回填环境的既有 checkpoint，不能用当前 shard 非空状态覆盖真实进度。
INSERT IGNORE INTO `video_comment_reaction_backfill_state` (`job_name`, `cursor_id`, `completed`)
SELECT 'reaction_shards_v1',
       IF(EXISTS (SELECT 1 FROM `video_comment_reaction_shards` LIMIT 1), COALESCE(MAX(`id`), 0), 0),
       EXISTS (SELECT 1 FROM `video_comment_reaction_shards` LIMIT 1)
FROM `video_comment_reactions`;

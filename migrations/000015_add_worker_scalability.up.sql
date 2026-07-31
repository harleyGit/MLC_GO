-- Stored buckets make fixed reprojection ranges sargable. The application never evaluates CRC32/MOD while scanning billion-row tables.
ALTER TABLE `video_user_interactions`
    ADD COLUMN `reproject_bucket` SMALLINT UNSIGNED GENERATED ALWAYS AS (MOD(CRC32(`submission_id`), 1024)) STORED,
    ADD INDEX `idx_video_interaction_reproject_bucket` (`reproject_bucket`, `updated_at`, `id`), ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE `user_follow_relations`
    ADD COLUMN `reproject_bucket` SMALLINT UNSIGNED GENERATED ALWAYS AS (MOD(CRC32(`followee_id`), 1024)) STORED,
    ADD INDEX `idx_follow_reproject_bucket` (`reproject_bucket`, `updated_at`, `id`), ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE `video_interaction_stat_shards`
    ADD COLUMN `reproject_bucket` SMALLINT UNSIGNED GENERATED ALWAYS AS (MOD(CRC32(`submission_id`), 1024)) STORED,
    ADD INDEX `idx_video_count_reproject_bucket` (`reproject_bucket`, `updated_at`, `submission_id`, `shard_id`), ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE `user_follow_stat_shards`
    ADD COLUMN `reproject_bucket` SMALLINT UNSIGNED GENERATED ALWAYS AS (MOD(CRC32(`user_id`), 1024)) STORED,
    ADD INDEX `idx_follow_count_reproject_bucket` (`reproject_bucket`, `updated_at`, `user_id`, `shard_id`), ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE `coin_asset_lots`
    ADD INDEX `idx_coin_lot_consolidation` (`user_id`, `remaining_amount`, `expires_sort`, `id`), ALGORITHM=INPLACE, LOCK=NONE;

-- Consolidation provenance uses explicit source/target allocation types longer than the original 16-character field.
ALTER TABLE `coin_asset_allocations`
    MODIFY COLUMN `allocation_type` VARCHAR(32) NOT NULL, ALGORITHM=INPLACE, LOCK=NONE;

CREATE TABLE IF NOT EXISTS `coin_lot_consolidation_links` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `consolidation_transaction_id` BIGINT UNSIGNED NOT NULL,
    `source_lot_id` BIGINT UNSIGNED NOT NULL,
    `target_lot_id` BIGINT UNSIGNED NOT NULL,
    `amount` BIGINT UNSIGNED NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_coin_consolidation_source` (`consolidation_transaction_id`, `source_lot_id`),
    KEY `idx_coin_consolidation_target` (`target_lot_id`, `source_lot_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='不可变 lot 合并来源到目标链接';

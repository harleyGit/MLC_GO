-- Migration 13 增加通用硬币权威资产核心，但不在 migration 内扫描 users。
-- 原因：亿级用户全量 INSERT ... SELECT 会形成长事务、binlog/复制压力和不可控发布时间。
-- 既有和未来用户通过在线惰性初始化或 users.id keyset 后台任务创建 wallet。
-- user_coin_wallets 继续作为余额快照；coin_asset_transactions/coin_asset_lots/coin_asset_allocations
-- 构成不可变审计链和到期来源。migration 12 的 user_coin_commands 保留只读兼容，用于旧投币幂等和额度。
CREATE TABLE IF NOT EXISTS `coin_asset_requests` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(255) NOT NULL,
    `request_id` VARCHAR(128) NOT NULL,
    `operation` VARCHAR(24) NOT NULL,
    `command_hash` CHAR(64) NOT NULL,
    `status` VARCHAR(16) NOT NULL,
    `transaction_id` BIGINT UNSIGNED NULL,
    `balance_after` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_coin_request` (`user_id`, `request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='硬币资产命令幂等记录';

CREATE TABLE IF NOT EXISTS `coin_asset_transactions` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(255) NOT NULL,
    `request_id` VARCHAR(128) NOT NULL,
    `operation` VARCHAR(24) NOT NULL,
    `amount` BIGINT UNSIGNED NOT NULL,
    `signed_delta` BIGINT NOT NULL,
    `balance_after` BIGINT UNSIGNED NOT NULL,
    `reason` VARCHAR(255) NOT NULL DEFAULT '',
    `business_type` VARCHAR(64) NOT NULL DEFAULT '',
    `business_key` VARCHAR(255) NOT NULL DEFAULT '',
    `reference_transaction_id` BIGINT UNSIGNED NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_coin_transaction_request` (`user_id`, `request_id`),
    KEY `idx_coin_transaction_user_created` (`user_id`, `created_at`, `id`),
    KEY `idx_coin_transaction_business` (`user_id`, `operation`, `business_type`, `business_key`, `id`),
    KEY `idx_coin_transaction_reference` (`reference_transaction_id`, `operation`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='不可变通用硬币资产流水';

CREATE TABLE IF NOT EXISTS `coin_asset_lots` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(255) NOT NULL,
    `source_transaction_id` BIGINT UNSIGNED NOT NULL,
    `original_amount` BIGINT UNSIGNED NOT NULL,
    `remaining_amount` BIGINT UNSIGNED NOT NULL,
    `expires_at` TIMESTAMP NULL,
    -- 永久 lot 排到所有有期限 lot 之后，使 debit 可直接按 expires_sort,id 执行 FEFO。
    `expires_sort` DATETIME GENERATED ALWAYS AS (COALESCE(`expires_at`, '9999-12-31 23:59:59')) STORED,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_coin_lot_source` (`source_transaction_id`),
    KEY `idx_coin_lot_fefo` (`user_id`, `expires_sort`, `id`, `remaining_amount`),
    KEY `idx_coin_lot_expiration` (`expires_at`, `id`, `user_id`, `remaining_amount`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='可过期硬币批次';

CREATE TABLE IF NOT EXISTS `coin_asset_allocations` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `transaction_id` BIGINT UNSIGNED NOT NULL,
    `lot_id` BIGINT UNSIGNED NOT NULL,
    `amount` BIGINT UNSIGNED NOT NULL,
    `allocation_type` VARCHAR(16) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_coin_allocation` (`transaction_id`, `lot_id`, `allocation_type`),
    KEY `idx_coin_allocation_lot` (`lot_id`, `transaction_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='流水到硬币批次的不可变分配';

CREATE TABLE IF NOT EXISTS `coin_job_checkpoints` (
    `job_name` VARCHAR(64) NOT NULL,
    `cursor_value` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`job_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='硬币后台任务游标';

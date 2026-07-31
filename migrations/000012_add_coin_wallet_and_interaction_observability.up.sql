CREATE TABLE IF NOT EXISTS `user_coin_wallets` (
    `user_id` VARCHAR(255) NOT NULL,
    `balance` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户硬币余额账户';

INSERT IGNORE INTO `user_coin_wallets` (`user_id`, `balance`)
SELECT `user_id`, 0 FROM `users`;

CREATE TABLE IF NOT EXISTS `user_coin_commands` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(255) NOT NULL,
    `request_id` VARCHAR(64) NOT NULL,
    `submission_id` VARCHAR(64) NOT NULL,
    `quantity` TINYINT UNSIGNED NOT NULL,
    `status` VARCHAR(16) NOT NULL DEFAULT 'processing',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_request` (`user_id`, `request_id`),
    KEY `idx_user_submission_status` (`user_id`, `submission_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='投币命令幂等与单视频额度预占';

CREATE TABLE IF NOT EXISTS `user_coin_ledger` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(255) NOT NULL,
    `request_id` VARCHAR(64) NOT NULL,
    `submission_id` VARCHAR(64) NOT NULL,
    `delta` BIGINT NOT NULL,
    `balance_after` BIGINT UNSIGNED NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_request` (`user_id`, `request_id`),
    KEY `idx_user_created` (`user_id`, `created_at`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户硬币不可变资产流水';

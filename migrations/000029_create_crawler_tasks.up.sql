-- Crawler task definitions retain editable scalar scheduling fields alongside flexible JSON configuration.
-- Both tables use monotonically increasing primary keys for stable cursor scans; no foreign keys are used.
CREATE TABLE `crawler_task_definitions` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(128) NOT NULL,
    `platform` VARCHAR(32) NOT NULL,
    `enabled` TINYINT(1) NOT NULL DEFAULT 0,
    `cron` VARCHAR(128) NOT NULL DEFAULT '',
    `parser_type` VARCHAR(32) NOT NULL,
    `item_path` VARCHAR(512) NOT NULL DEFAULT '',
    `max_items` INT UNSIGNED NOT NULL DEFAULT 0,
    `configuration` JSON NOT NULL,
    `last_run_id` BIGINT UNSIGNED NULL,
    `last_run_status` VARCHAR(32) NOT NULL DEFAULT '',
    `last_run_started_at` TIMESTAMP(6) NULL,
    `last_run_finished_at` TIMESTAMP(6) NULL,
    `last_run_item_count` INT UNSIGNED NOT NULL DEFAULT 0,
    `last_run_error` VARCHAR(2048) NOT NULL DEFAULT '',
    `version` BIGINT UNSIGNED NOT NULL DEFAULT 1,
    `created_by` VARCHAR(64) NOT NULL DEFAULT '',
    `updated_by` VARCHAR(64) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`id`),
    KEY `idx_crawler_task_definitions_enabled_id` (`enabled`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Crawler task definitions';

CREATE TABLE `crawler_task_runs` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `task_definition_id` BIGINT UNSIGNED NOT NULL,
    `status` VARCHAR(32) NOT NULL,
    `configuration` JSON NOT NULL,
    `started_at` TIMESTAMP(6) NOT NULL,
    `finished_at` TIMESTAMP(6) NULL,
    `item_count` INT UNSIGNED NOT NULL DEFAULT 0,
    `error_message` VARCHAR(2048) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`id`),
    KEY `idx_crawler_task_runs_definition_id` (`task_definition_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Crawler task execution runs';

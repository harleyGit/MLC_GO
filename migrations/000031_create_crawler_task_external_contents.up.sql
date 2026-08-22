-- Task ownership is normalized because crawler_external_contents is globally deduplicated by (platform, content_id).
-- The association primary key is a stable reverse cursor; no foreign keys are used to preserve current migration policy.
CREATE TABLE `crawler_task_external_contents` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `task_definition_id` BIGINT UNSIGNED NOT NULL,
    `external_content_id` BIGINT UNSIGNED NOT NULL,
    `last_run_id` BIGINT UNSIGNED NOT NULL,
    `created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_crawler_task_external_content` (`task_definition_id`, `external_content_id`),
    KEY `idx_crawler_task_external_contents_task_id` (`task_definition_id`, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Crawler task to globally deduplicated external content associations';

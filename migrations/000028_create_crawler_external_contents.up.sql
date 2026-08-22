-- crawler 仅保存第三方公开元数据和原站播放入口，不写入本站投稿/媒体文件表。
-- (platform, content_id) 是周期抓取和任务重试的幂等边界；列表按 last_seen_at + id 使用稳定游标。
CREATE TABLE `crawler_external_contents` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `platform` VARCHAR(32) NOT NULL,
    `content_id` VARCHAR(128) NOT NULL,
    `title` VARCHAR(255) NOT NULL,
    `author_id` VARCHAR(128) NOT NULL DEFAULT '',
    `author_name` VARCHAR(255) NOT NULL DEFAULT '',
    `cover_url` VARCHAR(1024) NOT NULL DEFAULT '',
    `target_url` VARCHAR(2048) NOT NULL,
    `duration_seconds` INT UNSIGNED NOT NULL DEFAULT 0,
    `view_count` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `like_count` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `comment_count` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `published_at` TIMESTAMP(6) NULL,
    `first_seen_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    `last_seen_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    `created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_crawler_external_content` (`platform`, `content_id`),
    KEY `idx_crawler_external_recent` (`last_seen_at` DESC, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='第三方平台公开内容元数据';

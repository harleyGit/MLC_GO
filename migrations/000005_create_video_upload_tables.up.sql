USE HG_MLC_DB;

-- ============================================
-- 000005: 视频投稿业务表
-- ============================================
-- 包含以下表：
--   1. video_submissions      稿件主表（一次投稿）
--   2. video_files            视频文件表（每个视频/分P）
--   3. video_tags             视频标签关联表
--   4. video_scheduled_publish 定时发布表
--   5. video_commercial_promotion 商业推广表
--   6. video_chapters        视频章节表
--   7. video_subtitles       视频字幕表
-- ============================================


-- ============================================
-- 1. 稿件主表：一次投稿的基本信息与全局设置
-- ============================================
-- 一次投稿可包含多个视频（分P），稿件级别的配置统一存此表。
CREATE TABLE IF NOT EXISTS `video_submissions`(
    `id` BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    `submission_id` VARCHAR(64) NOT NULL COMMENT '稿件唯一标识（业务ID）',
    `user_id` VARCHAR(255) NOT NULL COMMENT '投稿用户ID，关联 users.user_id',
    `title` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '稿件标题（取第一个视频标题或用户自定义）',
    `cover_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '稿件封面URL',
    `category` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '稿件分区',
    `video_type` VARCHAR(16) NOT NULL DEFAULT '自制' COMMENT '类型: 自制/转载',
    `source_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '转载来源URL（video_type=转载时必填）',
    `description` TEXT NULL COMMENT '稿件简介',
    `allow_secondary_creation` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否允许二创: 0否 1是',
    `watermark` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否添加水印: 0否 1是（仅本次上传有效）',
    `visibility` VARCHAR(16) NOT NULL DEFAULT 'public' COMMENT '可见范围: public公开/private仅自己',
    `declaration` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创作声明（如: ai/danger/entertainment/uncomfortable/consume/personal）',
    `card_config` JSON NULL COMMENT '个性化卡片配置（JSON）',
    `dolby_audio` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '杜比音效: 0否 1是',
    `hires_audio` TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Hi-Res无损音质: 0否 1是',
    `close_danmaku` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否关闭弹幕: 0否 1是',
    `close_comment` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否关闭评论: 0否 1是',
    `featured_comment` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否开启精选评论: 0否 1是',
    `dynamic_description` VARCHAR(233) NOT NULL DEFAULT '' COMMENT '粉丝动态描述，最多233字',
    `hide_from_profile` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否在个人空间-投稿中隐藏: 0否 1是',
    `video_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '包含的视频文件数量',
    `total_duration` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总时长（秒）',
    `total_size` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总文件大小（字节）',
    `status` VARCHAR(16) NOT NULL DEFAULT 'draft' COMMENT '稿件状态: draft草稿/reviewing审核中/published已发布/rejected已退回/archived已封存',
    `submit_time` TIMESTAMP NULL COMMENT '提交审核时间',
    `publish_time` TIMESTAMP NULL COMMENT '实际发布时间',
    `reject_reason` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '退回原因',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    UNIQUE KEY `uk_submission_id` (`submission_id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_status` (`status`),
    KEY `idx_category` (`category`),
    KEY `idx_created_at` (`created_at`)
)engine=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频稿件主表';


-- ============================================
-- 2. 视频文件表：每个视频/分P的文件信息与独立配置
-- ============================================
-- 一个稿件可包含多个视频文件（分P），每个视频有独立的上传状态和配置。
CREATE TABLE IF NOT EXISTS `video_files`(
    `id` BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    `video_id` VARCHAR(64) NOT NULL COMMENT '视频唯一标识（业务ID）',
    `submission_id` VARCHAR(64) NOT NULL COMMENT '所属稿件ID，关联 video_submissions.submission_id',
    `user_id` VARCHAR(255) NOT NULL COMMENT '上传用户ID，关联 users.user_id',
    `part_number` INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '分P序号（从1开始）',
    `title` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '视频标题（独立于稿件标题）',
    `cover_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '视频封面URL（可从视频帧选取）',
    `file_name` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '原始文件名',
    `file_path` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '存储路径（对象存储或本地路径）',
    `file_size` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件大小（字节）',
    `duration` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '视频时长（秒）',
    `mime_type` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'MIME类型（如 video/mp4）',
    `resolution_width` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '分辨率-宽',
    `resolution_height` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '分辨率-高',
    `bitrate` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '码率（kbps）',
    `md5` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '文件MD5校验值',
    `upload_status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT '上传状态: pending等待/uploading上传中/paused已暂停/completed已完成/failed失败',
    `upload_progress` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT '上传进度（0.00~100.00）',
    `transcode_status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT '转码状态: pending等待/transcoding转码中/completed已完成/failed失败',
    `video_type` VARCHAR(16) NOT NULL DEFAULT '自制' COMMENT '类型: 自制/转载',
    `source_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '转载来源URL',
    `category` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '视频分区',
    `description` TEXT NULL COMMENT '视频简介',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    UNIQUE KEY `uk_video_id` (`video_id`),
    UNIQUE KEY `uk_submission_part_number` (`submission_id`, `part_number`),
    KEY `idx_submission_id` (`submission_id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_upload_status` (`upload_status`),
    KEY `idx_md5` (`md5`)
)engine=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频文件表（分P）';


-- ============================================
-- 3. 视频标签关联表：多对多关系
-- ============================================
-- 每个视频最多7个标签，标签与视频为多对多关系。
CREATE TABLE IF NOT EXISTS `video_tags`(
    `id` BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    `video_id` VARCHAR(64) NOT NULL COMMENT '视频ID，关联 video_files.video_id',
    `tag_name` VARCHAR(32) NOT NULL COMMENT '标签名称',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    UNIQUE KEY `uk_video_tag` (`video_id`, `tag_name`),
    KEY `idx_tag_name` (`tag_name`)
)engine=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频标签关联表';


-- ============================================
-- 4. 定时发布表
-- ============================================
-- 稿件级别的定时发布配置，一次投稿对应一条定时发布记录（可选）。
CREATE TABLE IF NOT EXISTS `video_scheduled_publish`(
    `id` BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    `submission_id` VARCHAR(64) NOT NULL COMMENT '稿件ID，关联 video_submissions.submission_id',
    `user_id` VARCHAR(255) NOT NULL COMMENT '用户ID，关联 users.user_id',
    `scheduled_time` TIMESTAMP NOT NULL COMMENT '计划发布时间',
    `status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT '状态: pending待发布/published已发布/cancelled已取消/failed失败',
    `retry_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '重试次数',
    `max_retries` INT UNSIGNED NOT NULL DEFAULT 3 COMMENT '最大重试次数',
    `last_error` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '最后一次错误信息',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    UNIQUE KEY `uk_scheduled_submission` (`submission_id`),
    KEY `idx_scheduled_time_status` (`scheduled_time`, `status`),
    KEY `idx_user_id` (`user_id`)
)engine=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='定时发布表';


-- ============================================
-- 5. 商业推广表
-- ============================================
-- 稿件级别的商业推广配置，单稿件仅支持一种商业推广信息。
CREATE TABLE IF NOT EXISTS `video_commercial_promotion`(
    `id` BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    `submission_id` VARCHAR(64) NOT NULL COMMENT '稿件ID，关联 video_submissions.submission_id',
    `user_id` VARCHAR(255) NOT NULL COMMENT '用户ID，关联 users.user_id',
    `promotion_type` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '推广类型: 手机游戏/通用行业/主机游戏/网页游戏/PC单机游戏/PC网络游戏/软件及APP',
    `promotion_name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '推广名称',
    `promotion_form` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '推广形式: 定制软广/其他/口播/贴片/字幕推广/TVC植入/Logo/二维码/节目赞助/slogan',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    UNIQUE KEY `uk_commercial_submission` (`submission_id`),
    KEY `idx_user_id` (`user_id`)
)engine=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='商业推广配置表';


-- ============================================
-- 6. 视频章节表
-- ============================================
-- 每个视频可配置多个章节，用于视频内跳转。
CREATE TABLE IF NOT EXISTS `video_chapters`(
    `id` BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    `video_id` VARCHAR(64) NOT NULL COMMENT '视频ID，关联 video_files.video_id',
    `title` VARCHAR(128) NOT NULL COMMENT '章节标题',
    `start_time` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '章节起始时间（秒）',
    `sort_order` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '排序序号',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    UNIQUE KEY `uk_video_start_time` (`video_id`, `start_time`),
    KEY `idx_video_id` (`video_id`)
)engine=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频章节表';


-- ============================================
-- 7. 视频字幕表
-- ============================================
-- 每个视频可上传多语言字幕文件。
CREATE TABLE IF NOT EXISTS `video_subtitles`(
    `id` BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    `video_id` VARCHAR(64) NOT NULL COMMENT '视频ID，关联 video_files.video_id',
    `language` VARCHAR(16) NOT NULL DEFAULT 'zh' COMMENT '语言代码（如 zh/en/ja/ko）',
    `file_path` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '字幕文件路径',
    `file_name` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '原始文件名',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    UNIQUE KEY `uk_video_language` (`video_id`, `language`),
    KEY `idx_video_id` (`video_id`)
)engine=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频字幕表';

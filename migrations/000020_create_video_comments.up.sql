USE HG_MLC_DB;

-- 顶级评论当前同步写入；root/parent/reply 字段为后续回复能力保留。
-- 热表不建外键，关系完整性由服务写入校验维护，避免级联锁和大表 DDL 风险。
CREATE TABLE IF NOT EXISTS `video_comments` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `comment_id` VARCHAR(64) NOT NULL COMMENT '评论业务ID',
    `submission_id` VARCHAR(64) NOT NULL COMMENT '稿件业务ID',
    `user_id` VARCHAR(255) NOT NULL COMMENT '评论作者用户ID',
    `request_id` VARCHAR(64) NOT NULL COMMENT '用户维度请求幂等ID',
    `root_comment_id` VARCHAR(64) NULL COMMENT '根评论ID；顶级评论为空',
    `parent_comment_id` VARCHAR(64) NULL COMMENT '直接父评论ID；顶级评论为空',
    `reply_to_user_id` VARCHAR(255) NULL COMMENT '被回复用户ID；顶级评论为空',
    `content` VARCHAR(1000) NOT NULL COMMENT '评论内容，服务限制1到1000字符',
    `like_count` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '点赞数',
    `reply_count` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '回复数',
    `is_deleted` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '软删除标记',
    `created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    `deleted_at` TIMESTAMP(6) NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_video_comments_comment_id` (`comment_id`),
    UNIQUE KEY `uk_video_comments_user_request` (`user_id`, `request_id`),
    KEY `idx_video_comments_latest` (`submission_id`, `is_deleted`, `root_comment_id`, `created_at` DESC, `id` DESC),
    KEY `idx_video_comments_hot` (`submission_id`, `is_deleted`, `root_comment_id`, `like_count` DESC, `reply_count` DESC, `created_at` DESC, `id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频评论热表';

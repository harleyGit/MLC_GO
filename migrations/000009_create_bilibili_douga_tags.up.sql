USE HG_MLC_DB;

-- Bilibili 动画标签目录。
-- 设计边界：这里只保存可运营的标签目录，视频与标签的历史关联仍保存在 video_tags 大表，
-- 因此标签改名、停用或删除不会触发亿级关联表批量更新。
CREATE TABLE IF NOT EXISTS `bilibili_douga_tags` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `tag_id` VARCHAR(32) NOT NULL COMMENT '标签业务ID，BLTAG_加26位ULID',
    `name` VARCHAR(32) NOT NULL COMMENT '动画标签名称，推荐为前端保留字',
    `sort_order` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '升序展示顺序',
    `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '1启用 2停用',
    `is_deleted` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0正常 1删除',
    `deleted_token` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '软删除后写入id，释放活动名称唯一键',
    `created_by` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '创建人业务用户ID',
    `updated_by` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '最后更新人业务用户ID',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_tag_id` (`tag_id`),
    -- 正常记录 deleted_token 固定为 0，保证活动标签名称唯一；软删除后写入自身 id，从而允许重新创建同名标签。
    UNIQUE KEY `uk_name_deleted` (`name`, `deleted_token`),
    -- 运维列表使用 is_deleted=0 + id cursor 倒序翻页，禁止 OFFSET 深分页。
    KEY `idx_deleted_id` (`is_deleted`, `id`),
    -- 动画页按启用状态和 sort_order 读取，索引同时保证稳定的 id 次级排序。
    KEY `idx_active_sort_id` (`is_deleted`, `status`, `sort_order`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Bilibili动画标签目录';

-- 标签视频查询从 tag_name 驱动，并立即使用 video_id 关联 video_files。
-- 复合索引覆盖这两个字段，减少只命中旧 idx_tag_name 后的额外回表读取。
ALTER TABLE `video_tags`
    ADD KEY `idx_tag_name_video_id` (`tag_name`, `video_id`);

-- 初始化原动画页内置标签；INSERT IGNORE 使迁移重复执行时保持幂等。
-- “推荐”不入库，它在前端和 API 中表示 tagName 为空的无过滤列表。
INSERT IGNORE INTO `bilibili_douga_tags` (`tag_id`, `name`, `sort_order`, `status`) VALUES
    ('BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P', 'MAD·AMV', 10, 1),
    ('BLTAG_01K10D6JQS9XV3GR2F7B5M8N4Q', 'MMD·3D', 20, 1),
    ('BLTAG_01K10D6JQS9XV3GR2F7B5M8N4R', '短片·手书', 30, 1),
    ('BLTAG_01K10D6JQS9XV3GR2F7B5M8N4S', '配音·声优', 40, 1),
    ('BLTAG_01K10D6JQS9XV3GR2F7B5M8N4T', '特摄', 50, 1),
    ('BLTAG_01K10D6JQS9XV3GR2F7B5M8N4V', '综合', 60, 1);

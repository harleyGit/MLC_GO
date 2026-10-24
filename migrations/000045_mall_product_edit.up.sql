-- 已上架商品的文字资料编辑独立待审；目录/订单在审核前仍使用现有快照。
CREATE TABLE `mall_product_edit` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '审核队列倒序游标',
    `product_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '待修改商品业务标识，每商品仅保留最近一笔申请',
    `user_id` VARCHAR(255) NOT NULL COMMENT '申请编辑的店铺所有人业务标识',
    `title` VARCHAR(128) NOT NULL COMMENT '待审商品标题，最多128字符',
    `summary` VARCHAR(512) NOT NULL COMMENT '待审商品摘要，最多512字符纯文本',
    `description` VARCHAR(4096) NOT NULL COMMENT '待审商品介绍，最多4096字符纯文本',
    `status` TINYINT NOT NULL DEFAULT 0 COMMENT '0待审核、1通过、2拒绝',
    `reviewed_by` VARCHAR(255) DEFAULT NULL COMMENT '审核人业务标识，未审核时为空',
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '申请时间，UTC，毫秒精度',
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '最近状态更新时间，UTC，毫秒精度',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_mall_product_edit_product` (`product_id`),
    KEY `idx_mall_product_edit_status_id` (`status`,`id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='已上架商品文本编辑审核，公开目录审核前不变';

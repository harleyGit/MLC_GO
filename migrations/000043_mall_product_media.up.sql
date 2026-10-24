-- 仅增加商品介绍和有界图片引用；现有商品兼容空介绍、空图片。上线前评估 product 大表在线 DDL 窗口。
ALTER TABLE `product`
    ADD COLUMN `description` VARCHAR(4096) NOT NULL DEFAULT '' COMMENT '纯文本商品介绍，最多4096字符',
    ADD COLUMN `image_urls` JSON NULL COMMENT '最多5张经本店上传并审核的商品展示图片 URL，旧记录为空';

-- 上传先登记为 pending；只有上传完成且归属本商家用户的图片才能在发布事务内绑定。
CREATE TABLE `mall_product_image` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '内部自增主键',
    `image_url` VARCHAR(1024) NOT NULL COMMENT '上传成功后返回的公开图片 URL，唯一',
    `image_url_hash` BINARY(32) GENERATED ALWAYS AS (UNHEX(SHA2(`image_url`, 256))) STORED COMMENT 'URL 唯一索引的定长哈希，避免超长索引键',
    `storage_key` VARCHAR(512) NOT NULL COMMENT '配置化对象存储中的图片键',
    `user_id` VARCHAR(255) NOT NULL COMMENT '上传用户业务标识，发布时须同一用户',
    `product_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT '绑定的商品业务标识，未发布为空',
    `status` TINYINT NOT NULL DEFAULT 0 COMMENT '0上传中、1可绑定、2已绑定',
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '上传登记时间，UTC，毫秒精度',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_mall_product_image_url_hash` (`image_url_hash`),
    KEY `idx_mall_product_image_owner_status` (`user_id`,`status`,`id`),
    KEY `idx_mall_product_image_product` (`product_id`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

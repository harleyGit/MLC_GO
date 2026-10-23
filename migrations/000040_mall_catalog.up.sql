-- 商家准入及商品目录增量迁移；部署前确认 000037 已应用。新增列兼容旧商品行，旧行不自动对外公开。
CREATE TABLE `mall_merchant` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '商家内部自增主键，供审核游标使用',
    `merchant_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '商家业务标识，不暴露内部主键',
    `user_id` VARCHAR(255) NOT NULL COMMENT '店铺所有人用户业务标识，每用户至多一家店铺',
    `shop_name` VARCHAR(128) NOT NULL COMMENT '申请的店铺展示名称，最多128个字符',
    `status` TINYINT NOT NULL DEFAULT 0 COMMENT '入驻审核状态：0待审核、1通过、2拒绝',
    `reviewed_by` VARCHAR(255) DEFAULT NULL COMMENT '审核人用户业务标识；待审核时为空',
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '申请时间，UTC，毫秒精度',
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间，UTC，毫秒精度',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_mall_merchant_id` (`merchant_id`),
    UNIQUE KEY `uk_mall_merchant_user` (`user_id`),
    KEY `idx_mall_merchant_status_id` (`status`, `id` DESC),
    CONSTRAINT `ck_mall_merchant_status` CHECK (`status` IN (0, 1, 2))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='商家入驻与审核事实，不存储资金和个人证件';

ALTER TABLE `product`
    ADD COLUMN `merchant_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '发布商家业务标识；旧商品为空且不在新目录展示',
    ADD COLUMN `category` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '商品类目编码；旧商品为空，不在新目录展示',
    ADD COLUMN `summary` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '商品纯文本简介，最多512个字符，不解析 HTML',
    ADD KEY `idx_product_merchant_id` (`merchant_id`, `id` DESC),
    ADD KEY `idx_product_status_category_id` (`status`, `category`, `id` DESC);

-- 仅将新增 mall.merchant.review 授予既有生效的 super-admin，其他管理员须通过现有 RBAC 显式分配。
INSERT IGNORE INTO `permission` (`code`,`type`,`name`,`page_path`,`parent_id`,`status`,`sort`,`desc`,`create_at`,`update_at`,`update_by`) VALUES
('mall.merchant.review',2,'审核商家和商品','',-1,1,40,'审核商家入驻及商品发布',NOW(),NOW(),'migration-000040');
INSERT IGNORE INTO `role_permission` (`role_id`,`permission_id`,`create_by`,`update_by`)
SELECT r.`id`, p.`id`, 'migration-000040', 'migration-000040'
FROM `role` r JOIN `permission` p ON p.`code` = 'mall.merchant.review' AND p.`status` = 1
WHERE r.`name` = 'super-admin' AND r.`status` = 1;

-- 待支付订单单独保存收货快照；既有已支付订单和 mall_fulfillment 不回填、不改变。
-- 部署顺序：先应用本迁移，再启用待支付下单和到期库存释放任务。
CREATE TABLE `mall_pending_order` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '待支付订单内部递减游标，支付后从当前列表移除',
    `trade_order_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '交易订单业务标识，支付与过期释放均以此锁定',
    `user_id` VARCHAR(255) NOT NULL COMMENT '下单买家业务标识，仅本人可读取和支付',
    `merchant_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '下单时商家业务标识快照，支付后写入履约',
    `sku_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '已预留库存的单规格 SKU 业务标识',
    `quantity` INT UNSIGNED NOT NULL COMMENT '已预留商品件数，1至10件，到期释放一次',
    `receiver_name` VARCHAR(64) NOT NULL COMMENT '收件姓名快照，仅授权买家可读',
    `receiver_phone` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '收件电话快照，仅授权买家可读',
    `receiver_address` VARCHAR(512) NOT NULL COMMENT '详细收货地址快照，仅授权买家可读',
    `expire_at` DATETIME(3) NOT NULL COMMENT '库存预留与付款截止时间，UTC，毫秒精度，创建后15分钟',
    `reserved` TINYINT NOT NULL DEFAULT 1 COMMENT '库存是否仍预留：1待支付、0过期已释放；支付成功删除本快照',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_mall_pending_order` (`trade_order_id`),
    KEY `idx_mall_pending_user_id` (`user_id`, `id` DESC),
    KEY `idx_mall_pending_expire` (`reserved`, `expire_at`, `id`),
    CONSTRAINT `ck_mall_pending_quantity` CHECK (`quantity` BETWEEN 1 AND 10),
    CONSTRAINT `ck_mall_pending_reserved` CHECK (`reserved` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='商城待支付预留库存与买家收货快照，到期释放后保留取消记录';

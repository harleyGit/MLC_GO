-- 实物商品订单、库存与配送快照；仅在确认 000040 已部署且应用版本兼容时执行，禁止自动回填。
-- 旧商品审核状态为3(历史商品)，不更改 product.status 的停用/启用/归档语义；大表索引须预评估 DDL 锁与复制延迟。
ALTER TABLE `product`
    ADD COLUMN `review_status` TINYINT NOT NULL DEFAULT 3 COMMENT '商城审核状态：0待审、1通过、2拒绝、3旧商品无商城审核记录',
    ADD KEY `idx_product_review_id` (`review_status`, `id` DESC);

CREATE TABLE `mall_inventory` (
    `sku_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '单规格实物商品 SKU 业务标识，关联 product_sku.sku_id',
    `available` INT UNSIGNED NOT NULL COMMENT '可销售件数，下单同事务扣减，未发货退款同事务返还',
    `sold` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已售未退款件数，下单增加，未发货退款减少',
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '最近库存变化时间，UTC，毫秒精度',
    PRIMARY KEY (`sku_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='商城单规格库存权威记录';

CREATE TABLE `mall_fulfillment` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '内部递减游标主键',
    `trade_order_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '唯一订单业务标识，对应 trade_order.trade_order_id',
    `merchant_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '商家业务标识，物流权限按此归属校验',
    `user_id` VARCHAR(255) NOT NULL COMMENT '买家业务标识，列表与订单权限按此校验',
    `sku_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '购买时单规格 SKU 快照，退款返还库存使用',
    `payment_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '该订单唯一成功的 M币支付单业务标识，退款指向此记录',
    `quantity` INT UNSIGNED NOT NULL COMMENT '购买件数，首期限制1至10件',
    `debit_transaction_id` BIGINT UNSIGNED NOT NULL COMMENT '原始 M币扣减权威流水 ID，退款关联此笔流水',
    `receiver_name` VARCHAR(64) NOT NULL COMMENT '收件人姓名订单快照，仅买家和所属商家可读',
    `receiver_phone` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '收件手机号订单快照，仅买家和所属商家可读',
    `receiver_address` VARCHAR(512) NOT NULL COMMENT '详细收货地址订单快照，仅买家和所属商家可读',
    `shipping_status` TINYINT NOT NULL DEFAULT 0 COMMENT '物流状态：0待发货、1已发货、2已确认收货、3未发货退款',
    `carrier` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '商家填写承运商名称，未发货为空',
    `tracking_number` VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '商家填写运单号，未发货为空；不代表第三方轨迹已验证',
    `shipped_at` DATETIME(3) NULL COMMENT '商家提交运单时间，UTC，毫秒精度；未发货为空',
    `received_at` DATETIME(3) NULL COMMENT '买家确认收货时间，UTC，毫秒精度；未确认为空',
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '订单配送快照创建时间，UTC，毫秒精度',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_mall_fulfillment_order` (`trade_order_id`),
    UNIQUE KEY `uk_mall_fulfillment_payment` (`payment_id`),
    KEY `idx_mall_fulfillment_user_id` (`user_id`,`id` DESC),
    KEY `idx_mall_fulfillment_merchant_id` (`merchant_id`,`id` DESC),
    CONSTRAINT `ck_mall_shipping_status` CHECK (`shipping_status` BETWEEN 0 AND 3),
    CONSTRAINT `ck_mall_quantity` CHECK (`quantity` BETWEEN 1 AND 10)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='订单收货信息及人工录入运单，不存储外部物流轨迹';

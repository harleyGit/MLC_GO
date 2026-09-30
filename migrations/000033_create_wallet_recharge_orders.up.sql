-- 只新增订单快照与目录索引，不初始化支付渠道，不改变已有资产。
ALTER TABLE `payment_recharge_sku`
    ADD KEY `idx_wallet_sku_visible_id` (`is_deleted`, `status`, `id`);

CREATE TABLE `wallet_recharge_orders` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库内部自增主键',
    `order_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '服务端随机订单业务标识，不作为授权凭证',
    `user_id` VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '订单所属用户业务标识，由JWT取得',
    `request_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '用户范围创建幂等标识，区分大小写，永久绑定档位',
    `sku_id` VARCHAR(64) NOT NULL COMMENT '充值档位业务标识快照',
    `display_name` VARCHAR(255) NOT NULL COMMENT '服务端读取的用户展示名快照',
    `title` VARCHAR(128) NOT NULL COMMENT '服务端读取的充值档位标题快照',
    `description` VARCHAR(255) NOT NULL COMMENT '服务端生成的充值说明快照',
    `currency` CHAR(3) NOT NULL COMMENT '支付币种快照，当前仅支持人民币CNY',
    `pay_amount` BIGINT UNSIGNED NOT NULL COMMENT '服务端取得的应付金额快照，单位分',
    `total_coin` BIGINT UNSIGNED NOT NULL COMMENT '服务端取得的总币数快照，当前单笔最多1000币',
    `created_at` DATETIME(3) NOT NULL COMMENT '服务端UTC创建时间，精确到毫秒',
    `expires_at` DATETIME(3) NOT NULL COMMENT '服务端UTC到期时间，创建后10分钟；未到期为待支付，否则为已过期',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_wallet_order_id` (`order_id`),
    UNIQUE KEY `uk_wallet_order_request` (`user_id`, `request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='用户充值订单不可变快照，当前无支付和入账能力';

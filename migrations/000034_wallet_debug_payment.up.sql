-- 旧订单固定为unavailable，不允许日后变成模拟或真实支付订单；不得自动执行本迁移。
ALTER TABLE `wallet_recharge_orders`
    ADD COLUMN `payment_mode` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'unavailable' COMMENT '创建时绑定的支付模式：unavailable或platform_debug，禁止变更',
    ADD COLUMN `status` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending' COMMENT '持久状态pending或paid；未付款过期按到期时间计算',
    ADD COLUMN `paid_at` DATETIME(3) NULL COMMENT '模拟入账UTC时间；与coin资产同事务提交',
    ADD COLUMN `paid_transaction_id` BIGINT UNSIGNED NULL COMMENT '本次入账的coin权威流水编号',
    ADD COLUMN `balance_after` BIGINT UNSIGNED NULL COMMENT '支付完成时的权威余额快照，重放不读取当前余额';

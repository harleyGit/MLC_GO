-- 禁止已有paid数据时回退：删除支付审计标记可能导致误判，必须先停服并审计备份。
ALTER TABLE `wallet_recharge_orders`
    DROP COLUMN `balance_after`,
    DROP COLUMN `paid_transaction_id`,
    DROP COLUMN `paid_at`,
    DROP COLUMN `status`,
    DROP COLUMN `payment_mode`;

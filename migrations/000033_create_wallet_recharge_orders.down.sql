-- 回退会删除订单及幂等记录，已有业务数据时必须先评估并备份，禁止自动执行。
DROP TABLE `wallet_recharge_orders`;
ALTER TABLE `payment_recharge_sku` DROP INDEX `idx_wallet_sku_visible_id`;

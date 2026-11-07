-- 仅在确认无交易数据的隔离环境执行；生产已写入订单不可直接回滚。
DROP TABLE IF EXISTS hg_mall_cny_refund;
DROP TABLE IF EXISTS hg_mall_cny_payment;
DROP TABLE IF EXISTS hg_mall_cny_order;
DROP TABLE IF EXISTS hg_mall_cny_group;
DROP TABLE IF EXISTS hg_mall_cny_user_coupon;
DROP TABLE IF EXISTS hg_mall_cny_coupon;
DROP TABLE IF EXISTS hg_mall_cny_inventory;
DROP TABLE IF EXISTS hg_mall_cny_product;

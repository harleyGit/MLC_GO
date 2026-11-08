-- 仅在确认不再使用本店订单游标查询后，按正式迁移流程回退索引。
ALTER TABLE commerce_order DROP KEY idx_commerce_order_merchant_cursor;

-- 已应用的 000064 不修改原文件：本店订单跨多个状态时按 (merchant_id,id) 游标排序，避免大表 filesort。
ALTER TABLE commerce_order ADD KEY idx_commerce_order_merchant_cursor (merchant_id,id);

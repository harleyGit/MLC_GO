-- 商品补图审核按 status/id 有界游标读取；必须先应用 000043，不改动已有商品/订单状态。
ALTER TABLE `mall_product_image`
    MODIFY COLUMN `status` TINYINT NOT NULL DEFAULT 0 COMMENT '0上传中、1待使用、2已公开、3删除中、4待补图审核',
    ADD KEY `idx_mall_product_image_review_id` (`status`, `id` DESC);

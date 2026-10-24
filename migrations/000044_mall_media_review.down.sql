ALTER TABLE `mall_product_image`
    DROP KEY `idx_mall_product_image_review_id`,
    MODIFY COLUMN `status` TINYINT NOT NULL DEFAULT 0 COMMENT '0上传中、1待使用、2已公开、3删除中';

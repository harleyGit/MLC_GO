-- 仅当全部类目及关联 ID 仍为十进制正整数时可回退；存在自定义字符串 ID 时严格模式下应拒绝转换，避免静默丢数据。
ALTER TABLE commerce_product
 MODIFY COLUMN category_id BIGINT UNSIGNED NOT NULL COMMENT '创建时的可经营叶子类目 ID';

ALTER TABLE commerce_category_media
 MODIFY COLUMN category_id BIGINT UNSIGNED DEFAULT NULL COMMENT '绑定类目 ID；未绑定时为空';

ALTER TABLE commerce_category
 MODIFY COLUMN parent_id BIGINT UNSIGNED DEFAULT NULL COMMENT '上级类目 ID，空代表一级类目',
 MODIFY COLUMN id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '类目业务 ID，JSON 中按字符串传输';

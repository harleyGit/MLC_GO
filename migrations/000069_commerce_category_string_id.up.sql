-- 类目 ID 改为可读业务字符串；现有数字 ID 原样转为 ASCII 字符串，关联列保持同类型同排序规则。
ALTER TABLE commerce_category
 MODIFY COLUMN id BIGINT UNSIGNED NOT NULL COMMENT '迁移中间态：移除自增后转换为字符串业务 ID';

ALTER TABLE commerce_category
 MODIFY COLUMN id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '类目业务 ID，1-64 位安全 ASCII 字符',
 MODIFY COLUMN parent_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT '上级类目业务 ID；空代表一级类目';

ALTER TABLE commerce_product
 MODIFY COLUMN category_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '创建时的可经营叶子类目业务 ID';

ALTER TABLE commerce_category_media
 MODIFY COLUMN category_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT '绑定类目业务 ID；未绑定时为空';

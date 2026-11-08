-- 人民币生产商品库存盘点审计；与盘点更新同一事务，仅本机隔离 debug 获授权执行。
CREATE TABLE commerce_inventory_adjustment (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '审计游标',
 adjustment_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '服务端生成的盘点业务 ID',
 actor_user_id VARCHAR(255) NOT NULL COMMENT '授权操作者业务 ID',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '生产人民币商品 ID',
 sku_code VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '盘点 SKU 编码',
 delta BIGINT NOT NULL COMMENT '可售库存增减量，件',
 available_before BIGINT NOT NULL COMMENT '盘点前已核实可售库存，件',
 available_after BIGINT NOT NULL COMMENT '盘点后已核实可售库存，件',
 version_before BIGINT UNSIGNED NOT NULL COMMENT '盘点前版本',
 reason VARCHAR(256) NOT NULL COMMENT '人工盘点依据，非空',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '记录时间，UTC毫秒',
 PRIMARY KEY (id), UNIQUE KEY uk_commerce_adjustment_id (adjustment_id),
 KEY idx_commerce_adjustment_sku (product_id,sku_code,id),
 CONSTRAINT chk_commerce_adjustment_delta CHECK (delta<>0 AND delta BETWEEN -100000 AND 100000),
 CONSTRAINT chk_commerce_adjustment_balance CHECK (available_before>=0 AND available_after>=0 AND available_after=available_before+delta)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生产人民币 SKU 盘点不可变操作审计';

-- 生产 CNY 交易预留数据与 M币/模拟支付隔离；本迁移不授予任何支付、下单或发货路由。
CREATE TABLE commerce_inventory (
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '生产 CNY 商品业务 ID，唯一定位本商品库存',
 sku_code VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '跨审核版本稳定的本商品 SKU 编码，不采用每版本变化的 sku_id',
 available BIGINT NOT NULL DEFAULT 0 COMMENT '已核实可售件数，初始零，不从草稿建议库存自动入库',
 reserved BIGINT NOT NULL DEFAULT 0 COMMENT '待支付订单预占件数，仅同事务预占/释放',
 sold BIGINT NOT NULL DEFAULT 0 COMMENT '真实支付确认后成交件数，当前无渠道不得增加',
 version BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '库存操作乐观锁版本，每次调整或预占递增',
 updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '最近变更时间，UTC毫秒',
 PRIMARY KEY (product_id,sku_code),
 CONSTRAINT chk_commerce_inventory_counts CHECK (available>=0 AND reserved>=0 AND sold>=0 AND version>0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生产 CNY SKU 库存权威，跨商品审核版本保留预占';

CREATE TABLE commerce_order (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '订单列表稳定游标，禁止深 offset',
 trade_order_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '关联统一 trade_order 的唯一业务 ID，不与 M币订单混用',
 user_id VARCHAR(255) NOT NULL COMMENT '可信登录买家 user_id',
 merchant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '下单时已核准商家 ID，商家归属边界',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '下单时商品业务 ID 快照',
 sku_code VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '跨版本库存的 SKU 编码快照',
 sku_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '下单时公开版本 SKU ID 快照',
 title VARCHAR(120) NOT NULL COMMENT '下单时公开商品标题快照',
 specs_json JSON NOT NULL COMMENT '下单时公开 SKU 规格 JSON 快照',
 quantity INT UNSIGNED NOT NULL COMMENT '本单购买件数，1-10',
 unit_price_cent BIGINT NOT NULL COMMENT '下单时人民币单价，整数分',
 total_cent BIGINT NOT NULL COMMENT '下单时人民币应付总额，整数分，无折扣和运费',
 request_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '买家创建订单的稳定幂等键',
 request_hash BINARY(32) NOT NULL COMMENT '规范化商品、SKU、数量、地址 ID、预期价格的请求哈希',
 status TINYINT NOT NULL DEFAULT 1 COMMENT '1待支付、2已支付、3已发货、4完成、5取消；当前仅可创建1或关闭至5',
 address_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '用户地址业务 ID 快照来源，地址修改不覆盖订单',
 receiver_name VARCHAR(64) NOT NULL COMMENT '下单时收件姓名快照，仅授权买家和已支付的商家可读',
 receiver_phone VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '下单时收件手机号快照，列表永不返回',
 receiver_address VARCHAR(768) NOT NULL COMMENT '地区及详细地址快照，列表永不返回',
 expire_at DATETIME(3) NOT NULL COMMENT '待支付截止时间，UTC毫秒；过期释放需与支付事务竞争行锁',
 carrier VARCHAR(64) NOT NULL DEFAULT '' COMMENT '支付后人工登记的承运商；空表示未发货',
 tracking_number VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '支付后人工运单号；空表示未发货',
 shipped_at DATETIME(3) DEFAULT NULL COMMENT '已发货时间，UTC毫秒；未发货为空',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，UTC毫秒',
 updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间，UTC毫秒',
 PRIMARY KEY (id), UNIQUE KEY uk_commerce_order_trade (trade_order_id),
 UNIQUE KEY uk_commerce_order_request (user_id,request_id),
 KEY idx_commerce_order_user (user_id,id), KEY idx_commerce_order_merchant (merchant_id,status,id),
 KEY idx_commerce_order_expiry (status,expire_at,id),
 CONSTRAINT chk_commerce_order_quantity CHECK (quantity BETWEEN 1 AND 10),
 CONSTRAINT chk_commerce_order_amount CHECK (unit_price_cent>0 AND total_cent=unit_price_cent*quantity),
 CONSTRAINT chk_commerce_order_status CHECK (status BETWEEN 1 AND 5)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生产 CNY 订单事实及收件快照；支付渠道未接入时不得标记已付';

CREATE TABLE commerce_inventory_reservation (
 reservation_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '每订单唯一预占业务 ID，服务端生成',
 trade_order_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '生产 CNY 订单业务 ID，唯一约束保障释放不重复',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '预占时商品业务 ID，指向权威库存',
 sku_code VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '预占时 SKU 编码，跨审核版本可释放',
 quantity INT UNSIGNED NOT NULL COMMENT '预占件数，1-10',
 status TINYINT NOT NULL DEFAULT 1 COMMENT '1已预占、2已释放、3支付确认后转已售；无渠道不得转3',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '预占时间，UTC毫秒',
 updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '状态变更时间，UTC毫秒',
 PRIMARY KEY (reservation_id), UNIQUE KEY uk_commerce_reservation_order (trade_order_id),
 KEY idx_commerce_reservation_sku (product_id,sku_code,created_at),
 CONSTRAINT chk_commerce_reservation_qty CHECK (quantity BETWEEN 1 AND 10),
 CONSTRAINT chk_commerce_reservation_status CHECK (status IN (1,2,3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生产人民币库存预占台账，释放和支付确认必须原子变更状态';

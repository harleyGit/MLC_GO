-- 人民币商城与既有 MLC_COIN 订单严格隔离；仅写入迁移文件，部署时按授权环境执行。
CREATE TABLE hg_mall_cny_product (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '稳定目录游标',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '人民币商品业务 ID',
 merchant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '已审批商家业务 ID',
 title VARCHAR(128) NOT NULL COMMENT '公开商品标题',
 description VARCHAR(4096) NOT NULL DEFAULT '' COMMENT '纯文本商品介绍',
 image_url VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '已审核商品图片 URL；空为无图',
 sku_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '唯一在售 SKU ID',
 sku_title VARCHAR(128) NOT NULL COMMENT '唯一规格标题',
 price_fen BIGINT NOT NULL COMMENT '单买人民币单价，分',
 group_price_fen BIGINT DEFAULT NULL COMMENT '拼单人民币单价，分；空为不可拼',
 group_members TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '成团人数；0 为不可拼，2-10 为拼团',
 status TINYINT NOT NULL DEFAULT 1 COMMENT '1在售、2下架',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，UTC 毫秒',
 PRIMARY KEY (id), UNIQUE KEY uk_hg_cny_product (product_id), UNIQUE KEY uk_hg_cny_sku (sku_id),
 KEY idx_hg_cny_catalog (status,id), KEY idx_hg_cny_merchant (merchant_id,status,id),
 CONSTRAINT ck_hg_cny_price CHECK (price_fen > 0 AND (group_price_fen IS NULL OR (group_price_fen > 0 AND group_price_fen <= price_fen))),
 CONSTRAINT ck_hg_cny_group CHECK ((group_members=0 AND group_price_fen IS NULL) OR (group_members BETWEEN 2 AND 10 AND group_price_fen IS NOT NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='独立人民币商城商品（与 M币商品不互转）';
CREATE TABLE hg_mall_cny_inventory (
 sku_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '唯一规格 ID',
 available INT UNSIGNED NOT NULL COMMENT '可售库存，件',
 reserved INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '待支付预留库存，件',
 sold INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '模拟支付后已售库存，件',
 PRIMARY KEY (sku_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='人民币商品库存权威';
CREATE TABLE hg_mall_cny_coupon (
 coupon_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '优惠券模板 ID',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '适用商品业务 ID',
 threshold_fen BIGINT NOT NULL COMMENT '使用门槛，人民币分',
 discount_fen BIGINT NOT NULL COMMENT '优惠金额，人民币分',
 remaining INT UNSIGNED NOT NULL COMMENT '剩余可领取张数',
 expires_at DATETIME(3) NOT NULL COMMENT '领取和下单截止时间，UTC 毫秒',
 PRIMARY KEY (coupon_id), KEY idx_hg_cny_coupon_product (product_id,expires_at),
 CONSTRAINT ck_hg_cny_coupon CHECK (threshold_fen > 0 AND discount_fen > 0 AND discount_fen <= threshold_fen)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='人民币商城优惠券模板';
CREATE TABLE hg_mall_cny_user_coupon (
 user_id VARCHAR(255) NOT NULL COMMENT '领取用户业务 ID，来源 JWT',
 coupon_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '已领券模板 ID',
 order_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT '预占订单 ID；空表示可用',
 status TINYINT NOT NULL DEFAULT 0 COMMENT '0可用、1订单预占、2模拟付款核销',
 PRIMARY KEY (user_id,coupon_id), KEY idx_hg_cny_coupon_order (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='一人每券仅领取一次且事务核销';
CREATE TABLE hg_mall_cny_group (
 group_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '拼团业务 ID',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '关联人民币商品业务 ID',
 creator_id VARCHAR(255) NOT NULL COMMENT '团长用户业务 ID',
 target_members TINYINT UNSIGNED NOT NULL COMMENT '目标人数，2-10 人',
 paid_members TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已模拟支付人数',
 reserved_members TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '未支付占位人数',
 status TINYINT NOT NULL DEFAULT 0 COMMENT '0待团长支付、1招募中、2已成团、3到期失败、4团长取消',
 expires_at DATETIME(3) NOT NULL COMMENT '成团截止时间，UTC 毫秒',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，UTC 毫秒',
 PRIMARY KEY (group_id), KEY idx_hg_cny_groups_product (product_id,status,expires_at), KEY idx_hg_cny_groups_expire (status,expires_at,group_id),
 CONSTRAINT ck_hg_cny_group_slots CHECK (paid_members+reserved_members<=target_members)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='拼团付费及未付费席位权威';
CREATE TABLE hg_mall_cny_order (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '本人订单稳定游标',
 order_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '订单业务 ID',
 user_id VARCHAR(255) NOT NULL COMMENT '购买者业务 ID，来源 JWT',
 request_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '买家幂等请求 ID',
 request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '规范化请求 SHA-256',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '下单商品快照 ID',
 sku_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '下单规格快照 ID',
 title VARCHAR(128) NOT NULL COMMENT '下单时商品标题快照',
 quantity TINYINT UNSIGNED NOT NULL COMMENT '下单数量，1-10 件',
 purchase_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'single、group_create 或 group_join',
 group_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT '拼团 ID；单买为空',
 coupon_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT '锁定的优惠券 ID；拼团为空',
 total_fen BIGINT NOT NULL COMMENT '商品原价小计，人民币分',
 discount_fen BIGINT NOT NULL DEFAULT 0 COMMENT '优惠额，人民币分',
 payable_fen BIGINT NOT NULL COMMENT '应付金额，人民币分',
 address_snapshot VARCHAR(512) NOT NULL COMMENT '收货地址快照，不返回给其他买家',
 status TINYINT NOT NULL DEFAULT 0 COMMENT '0待模拟支付、1已付待发货、2等待成团、3取消、4退款完成',
 expires_at DATETIME(3) NOT NULL COMMENT '支付截止时间，UTC 毫秒',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，UTC 毫秒',
 paid_at DATETIME(3) DEFAULT NULL COMMENT '模拟支付时间，UTC 毫秒；未付为空',
 PRIMARY KEY (id), UNIQUE KEY uk_hg_cny_order (order_id), UNIQUE KEY uk_hg_cny_request (user_id,request_id),
 UNIQUE KEY uk_hg_cny_group_user (group_id,user_id), KEY idx_hg_cny_orders_user (user_id,id), KEY idx_hg_cny_orders_group (group_id,status), KEY idx_hg_cny_orders_expiry (status,expires_at,id),
 CONSTRAINT ck_hg_cny_order_amount CHECK (total_fen > 0 AND discount_fen >= 0 AND payable_fen > 0 AND total_fen-discount_fen=payable_fen)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='隔离的人民币分单位商城订单';
CREATE TABLE hg_mall_cny_payment (
 order_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '唯一模拟付款订单 ID',
 payment_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '模拟支付流水 ID',
 user_id VARCHAR(255) NOT NULL COMMENT '付款者业务 ID',
 amount_fen BIGINT NOT NULL COMMENT '模拟支付金额，人民币分；无真实资金',
 channel VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'alipay_sandbox' COMMENT '模拟渠道标识，不是支付宝交易号',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '模拟付款时间，UTC 毫秒',
 PRIMARY KEY (order_id), UNIQUE KEY uk_hg_cny_payment (payment_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='仅 debug 显式开启的模拟支付事实';
CREATE TABLE hg_mall_cny_refund (
 order_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '唯一全额模拟退款订单 ID',
 refund_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '模拟退款流水 ID',
 user_id VARCHAR(255) NOT NULL COMMENT '退款原支付用户业务 ID',
 amount_fen BIGINT NOT NULL COMMENT '模拟退款金额，人民币分；无真实资金',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '模拟退款时间，UTC 毫秒',
 PRIMARY KEY (order_id), UNIQUE KEY uk_hg_cny_refund (refund_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='仅模拟支付订单可用的全额退款事实';

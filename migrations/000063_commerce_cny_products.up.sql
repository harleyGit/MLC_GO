-- 生产 CNY 商品与现有 M币目录、debug CNY 沙箱隔离；未执行迁移前不可开放路由。
CREATE TABLE commerce_category (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '类目业务 ID，JSON 中按字符串传输',
 parent_id BIGINT UNSIGNED DEFAULT NULL COMMENT '上级类目 ID，空代表一级类目',
 depth TINYINT UNSIGNED NOT NULL COMMENT '层级，1-3',
 name VARCHAR(64) NOT NULL COMMENT '平台维护的类目名称',
 status TINYINT NOT NULL DEFAULT 1 COMMENT '1启用、0禁用',
 regulated TINYINT NOT NULL DEFAULT 0 COMMENT '1需经营资质，首期禁止发布；0为普通类目',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，UTC毫秒',
 PRIMARY KEY (id), KEY idx_commerce_category_parent (parent_id,status,id),
 CONSTRAINT chk_commerce_category_depth CHECK (depth BETWEEN 1 AND 3),
 CONSTRAINT chk_commerce_category_status CHECK (status IN (0,1) AND regulated IN (0,1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台动态类目树，初始为空，不自动开放受监管类目';

CREATE TABLE commerce_product (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '稳定商品列表游标',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '生产人民币商品唯一业务 ID',
 merchant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'mall_merchant 中已核准的商家 ID',
 category_id BIGINT UNSIGNED NOT NULL COMMENT '创建时的可经营叶子类目 ID',
 audit_status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'DRAFT' COMMENT 'DRAFT草稿、PENDING待审、APPROVED通过、REJECTED驳回',
 sale_status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'OFF_SHELF' COMMENT 'OFF_SHELF未上架、ON_SHELF在售、DELETED归档',
 version BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '乐观锁版本，每次状态或草稿更新递增',
 draft_version BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '本次待编辑或审核的内容版本',
 published_version BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已审核公开内容版本；0表示从未通过审核',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，UTC毫秒',
 updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间，UTC毫秒',
 PRIMARY KEY (id), UNIQUE KEY uk_commerce_product_id (product_id),
 KEY idx_commerce_product_merchant (merchant_id,id),
 KEY idx_commerce_product_category (category_id,sale_status,id),
 KEY idx_commerce_product_review (audit_status,id),
 CONSTRAINT chk_commerce_product_version CHECK (version>0 AND draft_version>0 AND published_version<=draft_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生产人民币商品状态及审核版本指针';

CREATE TABLE commerce_product_version (
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '所属生产人民币商品业务 ID',
 content_version BIGINT UNSIGNED NOT NULL COMMENT '商品内容版本号，从1递增',
 title VARCHAR(120) NOT NULL COMMENT '商品标题，最多120字符',
 subtitle VARCHAR(200) NOT NULL DEFAULT '' COMMENT '副标题，最多200字符',
 description VARCHAR(4096) NOT NULL DEFAULT '' COMMENT '纯文本介绍，最多4096字符，空表示没有介绍',
 images_json JSON NOT NULL COMMENT '本人已上传的展示图 URL 数组，1-5张，首张为主图',
 reviewed_by VARCHAR(255) DEFAULT NULL COMMENT '审核人用户 ID，未审核为空',
 reject_reason VARCHAR(256) NOT NULL DEFAULT '' COMMENT '驳回原因，非驳回时为空',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '版本创建时间，UTC毫秒',
 PRIMARY KEY (product_id,content_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核内容与公开内容隔离，历史版本不覆写';

CREATE TABLE commerce_sku (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '内部 SKU 序列',
 sku_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'SKU 唯一业务 ID；编辑时新版本生成新 ID',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '所属商品业务 ID',
 content_version BIGINT UNSIGNED NOT NULL COMMENT '所属商品内容版本号',
 sku_code VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '店铺内自定义 SKU 编码',
 specs_json JSON NOT NULL COMMENT '有界规格键值，最多三个维度',
 spec_hash BINARY(32) NOT NULL COMMENT '服务端规范化规格键值 SHA-256，防止同版本重复',
 price_cent BIGINT NOT NULL COMMENT '人民币单买价，整数分，禁止浮点金额',
 initial_stock BIGINT NOT NULL COMMENT '初始库存建议值，单位件；交易库存尚未启用，不可直接购买',
 PRIMARY KEY (id), UNIQUE KEY uk_commerce_sku_id (sku_id),
 UNIQUE KEY uk_commerce_sku_spec (product_id,content_version,spec_hash),
 UNIQUE KEY uk_commerce_sku_code (product_id,content_version,sku_code),
 KEY idx_commerce_sku_version (product_id,content_version,id),
 CONSTRAINT chk_commerce_sku_price CHECK (price_cent>0 AND initial_stock>=0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生产 CNY 商品审核版本 SKU，尚未连接订单库存';

CREATE TABLE commerce_media (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '上传额度与未使用图片游标',
 user_id VARCHAR(255) NOT NULL COMMENT '图片上传者的可信 JWT 用户 ID',
 image_url VARCHAR(1024) NOT NULL COMMENT '本服务对象存储生成的展示 URL',
 image_hash BINARY(32) GENERATED ALWAYS AS (UNHEX(SHA2(image_url,256))) STORED COMMENT 'URL 唯一定位哈希，避免长字符串索引',
 storage_key VARCHAR(1024) NOT NULL COMMENT '存储对象 key，只供服务端删除',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT '绑定商品 ID，未绑定时为空',
 status TINYINT NOT NULL DEFAULT 0 COMMENT '0上传中、1可绑定、2已绑定、3删除中',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '登记时间，UTC毫秒',
 PRIMARY KEY (id), UNIQUE KEY uk_commerce_media_hash (image_hash),
 KEY idx_commerce_media_owner (user_id,status,id), KEY idx_commerce_media_product (product_id,id),
 CONSTRAINT chk_commerce_media_status CHECK (status IN (0,1,2,3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生产 CNY 商家图片资产，独立于 M币和 debug 沙箱';

CREATE TABLE commerce_publish_request (
 merchant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '可信商家 ID，限制幂等键作用域',
 request_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '商家创建请求幂等键',
 request_hash BINARY(32) NOT NULL COMMENT '规范化业务正文 SHA-256',
 product_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '首次成功创建的商品 ID',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，UTC毫秒；保留用于安全重放',
 PRIMARY KEY (merchant_id,request_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生产人民币商品草稿创建幂等事实';

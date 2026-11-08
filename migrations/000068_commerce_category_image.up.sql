-- 类目图片可选；已存在的类目保持空图。类目图片与商家商品图片隔离，上传需商城审核权限。
ALTER TABLE commerce_category
 ADD COLUMN image_url VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '管理员上传并绑定的可选类目图片 URL；空为无图' AFTER name;

CREATE TABLE commerce_category_media_quota (
 user_id VARCHAR(255) NOT NULL COMMENT '可信管理员用户 ID；上传预留事务的串行锁键',
 PRIMARY KEY (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='每管理员图片配额并发预留锁，不持有对象数据';

CREATE TABLE commerce_category_media (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '管理员类目图片上传记录',
 user_id VARCHAR(255) NOT NULL COMMENT '上传者的可信 JWT 用户 ID',
 image_url VARCHAR(1024) NOT NULL COMMENT '服务端对象存储生成的 URL',
 image_hash BINARY(32) GENERATED ALWAYS AS (UNHEX(SHA2(image_url,256))) STORED COMMENT '定长 URL 定位哈希',
 storage_key VARCHAR(1024) NOT NULL COMMENT '对象存储 key，不返回给客户端',
 category_id BIGINT UNSIGNED DEFAULT NULL COMMENT '绑定类目 ID；未绑定时为空',
 status TINYINT NOT NULL DEFAULT 0 COMMENT '0 上传中、1 可绑定、2 已绑定、3 删除中',
 created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '上传登记时间，UTC 毫秒',
 PRIMARY KEY (id), UNIQUE KEY uk_commerce_category_media_hash (image_hash),
 KEY idx_commerce_category_media_owner (user_id,status,id),
 KEY idx_commerce_category_media_category (category_id,id),
 CONSTRAINT chk_commerce_category_media_status CHECK (status IN (0,1,2,3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台类目图片资产；与商家商品资产隔离';

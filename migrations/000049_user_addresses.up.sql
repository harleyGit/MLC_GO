-- 用户收货地址；发布前评估 users 外键建立时的锁与复制延迟，不自动回填。
CREATE TABLE `user_addresses` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '收货地址内部主键，按用户排序与定位',
    `address_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '收货地址全局唯一业务标识，由服务端生成，格式 ADDR_ 加26位 ULID',
    `user_id` VARCHAR(255) NOT NULL COMMENT '所属用户业务标识，关联 users.user_id',
    `receiver_name` VARCHAR(64) NOT NULL COMMENT '收件人姓名，非空，最多64字符',
    `receiver_phone` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '收件手机号，非空，11位中国大陆手机号',
    `region` VARCHAR(128) NOT NULL COMMENT '省市地区展示名称，非空，最多128字符',
    `province_code` VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '省级地区代码，6位数字，由客户端选择',
    `city_code` VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '市级地区代码，6位数字，由客户端选择',
    `detail_address` VARCHAR(512) NOT NULL COMMENT '街道门牌等详细地址，非空，最多512字符',
    `is_default` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否默认收货地址：0否、1是，每人最多一个',
    `default_user_id` VARCHAR(255) GENERATED ALWAYS AS (CASE WHEN `is_default` = 1 THEN `user_id` ELSE NULL END) STORED COMMENT '默认地址唯一约束辅助列；非默认地址为空',
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，UTC，毫秒精度',
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '修改时间，UTC，毫秒精度',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_addresses_address_id` (`address_id`),
    KEY `idx_user_addresses_user_id` (`user_id`, `id` DESC),
    UNIQUE KEY `uk_user_addresses_default_user` (`default_user_id`),
    CONSTRAINT `fk_user_addresses_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`user_id`) ON DELETE CASCADE,
    CONSTRAINT `ck_user_addresses_default` CHECK (`is_default` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户收货地址；写操作锁定 users 行串行化默认地址切换';

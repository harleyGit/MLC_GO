-- M币充值档位表
CREATE TABLE `payment_recharge_sku` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库内部自增主键，用于管理列表游标分页',
    `sku_id` VARCHAR(64) NOT NULL COMMENT '充值档位业务标识，由服务端生成，供接口及后续订单引用',
    `sku_code` VARCHAR(64) NOT NULL COMMENT '充值档位唯一业务编码，软删除后仍保留唯一约束',
    `title` VARCHAR(128) NOT NULL COMMENT '充值档位展示标题，最多128个字符',
    `currency` CHAR(3) NOT NULL DEFAULT 'CNY' COMMENT '支付币种，当前固定为人民币CNY',
    `pay_amount` BIGINT UNSIGNED NOT NULL COMMENT '实际支付金额，单位分',
    `coin_amount` BIGINT UNSIGNED NOT NULL COMMENT '获得平台币数量',
    `bonus_coin` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '额外赠送平台币数量',
    `total_coin` BIGINT UNSIGNED NOT NULL COMMENT '平台币总数量，由服务端计算为基础币与赠币之和',
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '档位启用状态：0停用，1启用',
    `sort_order` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '档位展示排序配置，当前管理列表仍按内部主键倒序分页',
    `start_time` DATETIME(3) NOT NULL COMMENT '档位生效时间，服务端按UTC写入，精确到毫秒',
    `end_time` DATETIME(3) DEFAULT NULL COMMENT '档位失效时间，服务端按UTC写入，NULL表示无截止时间',
    `version` INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '乐观锁版本号，从1开始，编辑及软删除成功后递增',
    `is_deleted` TINYINT NOT NULL DEFAULT 0 COMMENT '软删除标志：0未删除，1已删除；删除后保留历史记录',
    `created_by` VARCHAR(64) NOT NULL COMMENT '创建该档位的管理员用户业务标识',
    `updated_by` VARCHAR(64) NOT NULL COMMENT '最近编辑或软删除该档位的管理员用户业务标识',
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '记录创建时间，由数据库生成，精确到毫秒',
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '记录最近更新时间，由数据库自动维护，精确到毫秒',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_payment_recharge_sku_id` (`sku_id`),
    UNIQUE KEY `uk_payment_recharge_sku_code` (`sku_code`),
    KEY `idx_payment_recharge_sku_deleted_id` (`is_deleted`,`id` DESC),
    KEY `idx_payment_recharge_sku_status_sort` (`status`,`sort_order`,`id` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Payment recharge SKU catalog';

INSERT IGNORE INTO `permission` (`code`,`type`,`name`,`page_path`,`parent_id`,`status`,`sort`,`desc`,`create_at`,`update_at`,`update_by`) VALUES
('payment.recharge_sku.read',2,'Read recharge SKUs','',-1,1,40,'Read payment recharge SKU catalog',NOW(),NOW(),'migration-000032'),
('payment.recharge_sku.write',2,'Write recharge SKUs','',-1,1,41,'Create, update and delete payment recharge SKUs',NOW(),NOW(),'migration-000032');

INSERT IGNORE INTO `role_permission` (`role_id`,`permission_id`,`create_by`,`update_by`)
SELECT r.`id`, p.`id`, 'migration-000032', 'migration-000032'
FROM `role` r JOIN `permission` p ON p.`code` IN ('payment.recharge_sku.read','payment.recharge_sku.write') AND p.`status`=1
WHERE r.`name`='super-admin' AND r.`status`=1;

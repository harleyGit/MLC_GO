USE HG_MLC_DB;

-- 扩展 users 资料字段，保留原账号字段，避免影响现有登录与查询链路。
ALTER TABLE `users`
    ADD COLUMN `nickname` VARCHAR(64) NULL COMMENT '昵称' AFTER `user_name`,
    ADD COLUMN `signature` VARCHAR(255) NULL COMMENT '个性签名' AFTER `nickname`,
    ADD COLUMN `gender` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '性别: 0未知 1男 2女' AFTER `signature`,
    ADD COLUMN `birth_month` DATE NULL COMMENT '出生年月(统一存当月1号)' AFTER `gender`,
    ADD COLUMN `avatar_url` VARCHAR(512) NULL COMMENT '头像URL' AFTER `birth_month`;

-- 账号安全独立表：一个用户一条安全记录，支持绑定邮箱/手机/QQ/微信。
CREATE TABLE IF NOT EXISTS `user_security`(
    `id` BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    `user_id` VARCHAR(255) NOT NULL COMMENT '关联 users.user_id',
    `email` VARCHAR(255) NULL COMMENT '邮箱',
    `phone` VARCHAR(32) NULL COMMENT '手机号',
    `password_hash` VARCHAR(255) NOT NULL COMMENT '密码哈希值',
    `salt` VARCHAR(64) NOT NULL COMMENT '密码盐',
    `qq` VARCHAR(64) NULL COMMENT 'QQ号',
    `wechat` VARCHAR(128) NULL COMMENT '微信号',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    UNIQUE KEY `uk_user_security_user_id` (`user_id`),
    UNIQUE KEY `uk_user_security_email` (`email`),
    UNIQUE KEY `uk_user_security_phone` (`phone`),
    UNIQUE KEY `uk_user_security_qq` (`qq`),
    UNIQUE KEY `uk_user_security_wechat` (`wechat`),
    CONSTRAINT `fk_user_security_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`user_id`) ON DELETE CASCADE
)engine=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户账号安全表';

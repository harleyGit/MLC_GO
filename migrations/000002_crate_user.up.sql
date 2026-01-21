-- 还需要加的字段：昵称、用户ID号、头像、性别、地址
-- UNSIGEND： 表示该字段只允许非负数
-- COMMENT: 给字段添加注释
-- DEFAULT CURRENT_TIMESTAMP：当插入新记录且未指定 updated_at 值时，自动使用当前系统时间作为默认值。
-- ON UPDATE CURRENT_TIMESTAMP：每当该行被更新（UPDATE）时，自动将 updated_at 更新为当前时间。
CREATE TABLE IF NOT EXISTS `users`(
    `id` BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    `emai`l VARCHAR(255) NULL UNIQUE COMMENT '邮箱',
    `phone` VARCHAR(32) NULL UNIQUE COMMENT '手机号',
    `password_hash` VARCHAR(255) NOT NULL COMMENT '密码哈希值',
    `salt` VARCHAR(64) NOT NULL COMMENT '密码盐',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '创建时间',

    UNIQUE KEY `uk_email` (`email`),
    UNIQUE KEY `uk_phone` (`phone`)
)engine=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户表';
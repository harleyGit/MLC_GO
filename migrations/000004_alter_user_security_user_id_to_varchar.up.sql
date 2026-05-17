USE HG_MLC_DB;

-- 先补一列临时字符串字段，把旧的 users.id 映射成业务 user_id。
ALTER TABLE `user_security`
    ADD COLUMN `user_id_tmp` VARCHAR(255) NULL COMMENT '关联 users.user_id' AFTER `id`;

UPDATE `user_security` AS us
INNER JOIN `users` AS u ON u.`id` = us.`user_id`
SET us.`user_id_tmp` = u.`user_id`;

-- 替换旧整型列并重建外键/唯一索引。
ALTER TABLE `user_security`
    DROP FOREIGN KEY `fk_user_security_user_id`;

ALTER TABLE `user_security`
    DROP INDEX `uk_user_security_user_id`;

ALTER TABLE `user_security`
    DROP COLUMN `user_id`,
    CHANGE COLUMN `user_id_tmp` `user_id` VARCHAR(255) NOT NULL COMMENT '关联 users.user_id';

ALTER TABLE `user_security`
    ADD UNIQUE KEY `uk_user_security_user_id` (`user_id`),
    ADD CONSTRAINT `fk_user_security_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`user_id`) ON DELETE CASCADE;

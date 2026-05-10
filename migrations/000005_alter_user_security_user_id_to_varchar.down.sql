USE HG_MLC_DB;

ALTER TABLE `user_security`
    ADD COLUMN `user_id_tmp` BIGINT UNSIGNED NULL COMMENT '关联 users.id' AFTER `id`;

UPDATE `user_security` AS us
INNER JOIN `users` AS u ON u.`user_id` = us.`user_id`
SET us.`user_id_tmp` = u.`id`;

ALTER TABLE `user_security`
    DROP FOREIGN KEY `fk_user_security_user_id`;

ALTER TABLE `user_security`
    DROP INDEX `uk_user_security_user_id`;

ALTER TABLE `user_security`
    DROP COLUMN `user_id`,
    CHANGE COLUMN `user_id_tmp` `user_id` BIGINT UNSIGNED NOT NULL COMMENT '关联 users.id';

ALTER TABLE `user_security`
    ADD UNIQUE KEY `uk_user_security_user_id` (`user_id`),
    ADD CONSTRAINT `fk_user_security_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE;

USE HG_MLC_DB;

-- 迁移职责：把手机号 17681317668 对应的业务用户幂等提升为 super-admin，并同步登录、安全资料和角色权限。
-- 所有定位条件均使用手机号、业务 user_id、角色名或关联唯一键，不扫描大表，也不使用 OFFSET。

-- 已运行过旧版 000002 的环境可能已有该手机号；这里只重置登录密码，不改业务 user_id 和用户资料。
UPDATE `users`
SET `password_hash` = '19149107dec71205435b4f6b76b43554c9a20794e6211dc1ccab594df349fe27',
    `salt` = 'HGSA1768'
WHERE `phone` = '17681317668';

-- 兜底创建用户。手机号唯一索引保证点查和并发重复执行不会产生第二个账号。
INSERT INTO `users` (`user_id`, `user_name`, `email`, `phone`, `password_hash`, `salt`)
SELECT 'hgid_super_admin_17681317668', 'super_admin_17681317668', NULL, '17681317668',
       '19149107dec71205435b4f6b76b43554c9a20794e6211dc1ccab594df349fe27', 'HGSA1768'
WHERE NOT EXISTS (SELECT 1 FROM `users` WHERE `phone` = '17681317668');

-- 账号安全页以 user_security 为权威展示来源，和 users 同步密码及手机号，避免后续资料更新读到旧哈希。
INSERT INTO `user_security` (`user_id`, `email`, `phone`, `password_hash`, `salt`, `qq`, `wechat`)
SELECT u.`user_id`, u.`email`, u.`phone`, u.`password_hash`, u.`salt`, NULL, NULL
FROM `users` u
WHERE u.`phone` = '17681317668'
ON DUPLICATE KEY UPDATE
    `user_id` = VALUES(`user_id`),
    `phone` = VALUES(`phone`),
    `password_hash` = VALUES(`password_hash`),
    `salt` = VALUES(`salt`);

-- 角色名是授权链路的稳定标识；已存在时只恢复启用状态，不替换其内部主键。
INSERT INTO `role` (`role_id`, `name`, `description`, `status`, `create_at`, `update_at`, `create_by`, `update_by`)
VALUES ('ROL_SUPER_ADMIN_BOOTSTRAP', 'super-admin', 'System bootstrap super administrator', 1, NOW(), NOW(), 'migration-000019', 'migration-000019')
ON DUPLICATE KEY UPDATE
    `status` = 1,
    `description` = VALUES(`description`),
    `update_by` = 'migration-000019';

-- admin_user.mobile 唯一键把已有管理员和新建管理员收敛到同一行；user_id 始终同步 users 业务 ID。
INSERT INTO `admin_user` (`user_id`, `name`, `nick_name`, `email`, `mobile`, `lark_open_id`, `password`, `status`, `create_at`, `update_at`, `create_by`, `update_by`, `sex`, `is_delete`)
SELECT u.`user_id`, COALESCE(NULLIF(u.`user_name`, ''), 'Super Admin'), COALESCE(NULLIF(u.`nickname`, ''), NULLIF(u.`user_name`, ''), 'Super Admin'),
       NULL, u.`phone`, '', u.`password_hash`, 1, NOW(), NOW(), 'migration-000019', 'migration-000019', 3, 0
FROM `users` u
WHERE u.`phone` = '17681317668'
ON DUPLICATE KEY UPDATE
    `user_id` = VALUES(`user_id`),
    `password` = VALUES(`password`),
    `status` = 1,
    `is_delete` = 0,
    `update_by` = 'migration-000019';

-- 关联表具备 (admin_user_id, role_id) 唯一键，可安全重复执行。
INSERT IGNORE INTO `admin_user_role` (`admin_user_id`, `role_id`, `update_at`, `update_by`)
SELECT au.`id`, r.`id`, NOW(), 'migration-000019'
FROM `admin_user` au
JOIN `role` r ON r.`name` = 'super-admin' AND r.`status` = 1
WHERE au.`mobile` = '17681317668' AND au.`status` = 1 AND au.`is_delete` = 0;

-- role_permission 没有复合唯一键，使用 NOT EXISTS 防止重复迁移放大关联行。
INSERT INTO `role_permission` (`role_id`, `permission_id`, `create_by`, `update_by`)
SELECT r.`id`, p.`id`, 'migration-000019', 'migration-000019'
FROM `role` r
JOIN `permission` p ON p.`status` = 1
WHERE r.`name` = 'super-admin' AND r.`status` = 1
  AND NOT EXISTS (
      SELECT 1 FROM `role_permission` rp
      WHERE rp.`role_id` = r.`id` AND rp.`permission_id` = p.`id`
  );

-- 同步角色读优化快照，保证后台角色页和权限判断展示一致。
INSERT INTO `user_role_view` (`admin_user_id`, `role_id`, `role_name`, `status`, `create_at`)
SELECT au.`id`, r.`role_id`, r.`name`, 1, NOW()
FROM `admin_user` au
JOIN `role` r ON r.`name` = 'super-admin' AND r.`status` = 1
WHERE au.`mobile` = '17681317668' AND au.`status` = 1 AND au.`is_delete` = 0
ON DUPLICATE KEY UPDATE
    `role_name` = VALUES(`role_name`),
    `status` = 1;

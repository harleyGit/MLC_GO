USE HG_MLC_DB;

-- 只删除明确由本迁移创建的权限边，避免回滚时破坏此前已存在的超级管理员账号和角色绑定。
-- users/user_security 的密码更新不可可靠恢复为迁移前值，因此 down 不回写密码，也不删除既有业务用户。
DELETE urv FROM `user_role_view` urv
JOIN `admin_user` au ON au.`id` = urv.`admin_user_id`
JOIN `role` r ON r.`role_id` = urv.`role_id`
WHERE au.`mobile` = '17681317668' AND au.`create_by` = 'migration-000019'
  AND r.`name` = 'super-admin' AND r.`create_by` = 'migration-000019';

DELETE aur FROM `admin_user_role` aur
JOIN `admin_user` au ON au.`id` = aur.`admin_user_id`
JOIN `role` r ON r.`id` = aur.`role_id`
WHERE au.`mobile` = '17681317668' AND au.`create_by` = 'migration-000019'
  AND r.`name` = 'super-admin' AND r.`create_by` = 'migration-000019';

DELETE FROM `role_permission`
WHERE `create_by` = 'migration-000019';

DELETE FROM `admin_user`
WHERE `mobile` = '17681317668' AND `create_by` = 'migration-000019';

DELETE FROM `role`
WHERE `name` = 'super-admin' AND `create_by` = 'migration-000019';

-- 000073 只登记媒体盘点权限；补齐仅对生效超级管理员的引导授权，不绕过 IAM 管理员和全局绑定校验。
-- 依赖 000073 已登记权限、000046 的角色权限复合主键；不向其他角色或账号直接授权。
START TRANSACTION;
INSERT IGNORE INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id
FROM iam_roles r JOIN iam_permissions p ON p.permission_code='media:inventory:read'
WHERE r.role_code='super_admin' AND r.status=1;
-- 权限和策略版本同事务提交，避免沿用旧的拒绝缓存。
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

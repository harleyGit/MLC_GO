-- 000057 只登记权限；为避免无人持有新权限而无法经 IAM 委派，仅对内置超级管理员补齐引导授权。
-- 不给普通管理员或旧 RBAC 自动扩权。依赖 000057 权限存在、000046 的角色权限复合主键。
START TRANSACTION;
INSERT IGNORE INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id
FROM iam_roles r JOIN iam_permissions p ON p.permission_code='user:directory:write'
WHERE r.role_code='super_admin' AND r.status=1;
-- 与授权写入同事务推进策略版本，使已缓存的权限判定失效。
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

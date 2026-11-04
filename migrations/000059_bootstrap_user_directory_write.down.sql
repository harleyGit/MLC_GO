-- 先回退依赖该权限的应用代码；回滚会移除超级管理员对此权限的授权（含后续人工改动），需先备份授权记录。
START TRANSACTION;
DELETE rp FROM iam_role_permissions rp
JOIN iam_roles r ON r.id=rp.role_id
JOIN iam_permissions p ON p.id=rp.permission_id
WHERE r.role_code='super_admin' AND p.permission_code='user:directory:write';
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

-- 回退前须核对授权记录；此操作也会移除之后人工授予超级管理员的同一权限，不删除权限目录或其他角色授权。
START TRANSACTION;
DELETE rp FROM iam_role_permissions rp
JOIN iam_roles r ON r.id=rp.role_id
JOIN iam_permissions p ON p.id=rp.permission_id
WHERE r.role_code='super_admin' AND p.permission_code='media:inventory:read';
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

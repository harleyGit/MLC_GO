-- 回滚前应保存审批产生的角色授权；先回退应用代码，再移除该权限。
START TRANSACTION;
DELETE rp FROM iam_role_permissions rp
JOIN iam_permissions p ON p.id=rp.permission_id
WHERE p.permission_code='user:directory:write';
DELETE FROM iam_permissions WHERE permission_code='user:directory:write';
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

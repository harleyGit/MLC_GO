-- 回滚撤销操作权限；先前提交的清理任务和审计数据由原生命周期管理。
START TRANSACTION;
DELETE rp FROM iam_role_permissions rp JOIN iam_permissions p ON p.id=rp.permission_id
WHERE p.permission_code IN ('media:asset:preview','media:asset:delete');
DELETE FROM iam_permissions WHERE permission_code IN ('media:asset:preview','media:asset:delete');
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

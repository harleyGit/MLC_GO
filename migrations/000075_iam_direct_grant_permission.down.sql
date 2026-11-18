-- 回退即时授权能力时移除全部角色的该权限；已生效绑定需另行审计，不自动撤销其他业务授权。
DELETE rp FROM iam_role_permissions rp JOIN iam_permissions p ON p.id=rp.permission_id
WHERE p.permission_code='iam:role:grant_direct';
DELETE FROM iam_permissions WHERE permission_code='iam:role:grant_direct';
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;

-- 预览和删除独立于盘点权限；仅为生效的超级管理员引导授权，不给普通盘点角色加权。
START TRANSACTION;
INSERT INTO iam_permissions(permission_code,description) VALUES
('media:asset:preview','预览符合归属、用途和审核要求的新媒体'),
('media:asset:delete','申请异步清理单个无引用的新媒体');
INSERT INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id
FROM iam_roles r JOIN iam_permissions p ON p.permission_code IN ('media:asset:preview','media:asset:delete')
WHERE r.role_code='super_admin' AND r.status=1;
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

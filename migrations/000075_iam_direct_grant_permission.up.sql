-- 即时授权是独立高危能力；仅将权限预置给内置 super_admin，不给普通角色或旧 RBAC 自动继承。
-- 不修改已有双人审批申请；实际操作者仍须拥有有效全局绑定、本人 MFA 与可委派权限。
INSERT INTO iam_permissions(permission_code,description)
VALUES ('iam:role:grant_direct','对他人即时授予非超级管理员角色（需本人 MFA）');

INSERT INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r JOIN iam_permissions p ON p.permission_code='iam:role:grant_direct'
WHERE r.role_code='super_admin' AND r.status=1;

UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;

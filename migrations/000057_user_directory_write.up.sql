-- 先发布权限字典，再部署跨用户编辑代码；不自动授予任何角色，由 IAM 审批流程授权。
START TRANSACTION;
INSERT INTO iam_permissions(permission_code,description)
VALUES ('user:directory:write','修改其他用户资料及账号安全字段');
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

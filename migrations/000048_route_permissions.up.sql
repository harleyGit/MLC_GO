-- 路由级 IAM 权限字典。只处理 7 个权限和唯一编码定位的内置角色，不扫描用户/业务表。
-- 先部署迁移再部署代码；普通角色由既有 IAM 双人审批流程按需授权，禁止自动扩权。
-- 依赖 uk_iam_permission_code、uk_iam_role_code 和角色权限复合主键；不在路由注册时写库。
START TRANSACTION;
INSERT INTO iam_permissions(permission_code,description) VALUES
('ops:menu:read','读取运维菜单'),
('ops:menu:write','管理运维菜单'),
('ops:file:read','读取运维文件列表'),
('ops:file:write','上传运维文件'),
('ops:bilibili_tag:read','读取 Bilibili 动画标签配置'),
('ops:bilibili_tag:write','管理 Bilibili 动画标签配置'),
('user:directory:read','读取平台用户目录');

INSERT INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r JOIN iam_permissions p ON p.permission_code IN (
  'ops:menu:read','ops:menu:write','ops:file:read','ops:file:write',
  'ops:bilibili_tag:read','ops:bilibili_tag:write','user:directory:read'
) WHERE r.role_code='super_admin';

-- 同事务推进策略版本，让既有 Redis 权限缓存失效。
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

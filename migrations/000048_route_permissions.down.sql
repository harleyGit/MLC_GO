-- 先回退应用代码；回滚会删除这 7 项权限的角色关联，包括上线后人工授予的关联。
-- 仅按唯一权限编码定位，禁止扩大删除范围；回滚前由发布流程保存必要授权记录。
START TRANSACTION;
DELETE rp FROM iam_role_permissions rp
JOIN iam_permissions p ON p.id=rp.permission_id
WHERE p.permission_code IN (
  'ops:menu:read','ops:menu:write','ops:file:read','ops:file:write',
  'ops:bilibili_tag:read','ops:bilibili_tag:write','user:directory:read'
);
DELETE FROM iam_permissions WHERE permission_code IN (
  'ops:menu:read','ops:menu:write','ops:file:read','ops:file:write',
  'ops:bilibili_tag:read','ops:bilibili_tag:write','user:directory:read'
);
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

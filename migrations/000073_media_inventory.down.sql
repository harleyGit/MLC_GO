-- 回退应用入口及角色绑定后才能移除；不会触碰媒体对象或业务引用。
START TRANSACTION;
DELETE rp FROM iam_role_permissions rp JOIN iam_permissions p ON p.id=rp.permission_id WHERE p.permission_code='media:inventory:read';
DELETE FROM iam_permissions WHERE permission_code='media:inventory:read';
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;
ALTER TABLE users DROP KEY idx_users_avatar_inventory, DROP COLUMN hg_has_avatar;
ALTER TABLE im_private_attachment DROP KEY idx_im_inventory_type;
ALTER TABLE media_assets DROP KEY idx_media_inventory_kind, DROP COLUMN media_kind;

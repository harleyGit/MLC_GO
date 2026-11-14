-- 仅为有界只读盘点添加索引；大表在线 DDL 必须单独评估锁、磁盘与复制延迟，禁止直接在共享/生产库执行。
-- 不回填、不移动旧对象，也不赋予任何管理员访问对象字节的能力。
ALTER TABLE media_assets
  ADD COLUMN media_kind VARCHAR(8) GENERATED ALWAYS AS (CASE
    WHEN mime_type LIKE 'image/%' THEN 'image'
    WHEN mime_type LIKE 'video/%' THEN 'video'
    WHEN mime_type LIKE 'audio/%' THEN 'audio'
    ELSE 'file' END) VIRTUAL COMMENT '管理盘点分类：image/video/audio/file，仅由声明 MIME 派生，不代表可信审核',
  ADD KEY idx_media_inventory_kind (media_kind, asset_id);
ALTER TABLE im_private_attachment ADD KEY idx_im_inventory_type (media_type, id);
ALTER TABLE users
  ADD COLUMN hg_has_avatar TINYINT GENERATED ALWAYS AS (IF(avatar_url IS NOT NULL AND avatar_url <> '', 1, 0)) VIRTUAL COMMENT '旧头像盘点标记：1 存在非空 URL，0 不存在；不是审核状态',
  ADD KEY idx_users_avatar_inventory (hg_has_avatar, id);
START TRANSACTION;
INSERT INTO iam_permissions(permission_code,description)
VALUES ('media:inventory:read','只读盘点新旧媒体元数据，不含私有对象与文件字节');
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

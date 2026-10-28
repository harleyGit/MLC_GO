-- IAM 权威切换：旧 ops RBAC 只用于一次性回填，运行时不得再写旧角色表。
-- 部署前先清点旧表中非标准权限编码；未映射的编码不会获得授权。
CREATE TABLE iam_policy_revision (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  CHECK (id=1)
) ENGINE=InnoDB COMMENT='IAM 全局策略版本；授权变更同事务递增，Redis 缓存先核对版本';
INSERT INTO iam_policy_revision(id,revision) VALUES(1,1);

CREATE TABLE iam_mfa_factors (
  user_id VARCHAR(64) NOT NULL PRIMARY KEY,
  secret_cipher VARBINARY(255) NOT NULL COMMENT 'AES-GCM 加密的 TOTP 密钥',
  enabled TINYINT(1) NOT NULL DEFAULT 0,
  last_counter BIGINT NOT NULL DEFAULT -1 COMMENT '已消费的 TOTP 时间步，防止重放',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB COMMENT='IAM 管理员 TOTP 因子';

CREATE TABLE iam_approval_requests (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  request_id VARCHAR(64) NOT NULL,
  applicant_id VARCHAR(64) NOT NULL,
  approver_id VARCHAR(64) NOT NULL DEFAULT '',
  action VARCHAR(32) NOT NULL COMMENT 'grant 或 role_permissions',
  payload JSON NOT NULL COMMENT '服务端复核的变更参数；不包含凭据',
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  expires_at DATETIME(3) NOT NULL,
  decided_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_iam_approval_request(request_id),
  KEY idx_iam_approval_status_id(status,id),
  KEY idx_iam_approval_applicant_id(applicant_id,id)
) ENGINE=InnoDB COMMENT='双人审批；仅 pending 可裁决，申请人不能自批';

INSERT IGNORE INTO iam_roles(role_code,role_name,description,status,built_in)
SELECT CONCAT('legacy_',r.id),LEFT(r.name,100),LEFT(r.description,255),1,0
FROM role r WHERE r.status=1 AND r.name<>'super-admin';

-- 映射运行时实际使用的旧权限码；四段及以上编码显式归并到三段 IAM 协议。
INSERT IGNORE INTO iam_permissions(permission_code,description) VALUES
('asset:coin_balance:read','读取硬币余额'),('asset:coin_transaction:read','读取硬币交易'),
('asset:coin:grant','发放硬币'),('asset:coin:refund','硬币退款'),
('asset:coin_correction:request','申请硬币校正'),('asset:coin_correction:approve','审批硬币校正'),
('asset:coin_correction:apply','执行硬币校正'),('asset:pipeline:read','读取资产链路'),
('mall:merchant:review','审核商家'),('mall:product:review','审核商品'),
('crawler:task:read','读取爬虫任务'),('crawler:task:write','修改爬虫任务'),
('crawler:task:run','执行爬虫任务'),('ops:rbac:manage','管理旧 RBAC'),
('payment:recharge_sku:read','读取充值档位'),('payment:recharge_sku:write','管理充值档位'),
('video:submission:create','创建视频投稿'),('mall:product:publish','发布商品'),
('mall:order:read','读取本店订单'),('mall:order:ship','发货本店订单'),
('iam:approval:approve','审批 IAM 扩权');

INSERT IGNORE INTO iam_role_permissions(role_id,permission_id)
SELECT ir.id,ip.id FROM role r
JOIN iam_roles ir ON ir.role_code=IF(r.name='super-admin','super_admin',CONCAT('legacy_',r.id))
JOIN role_permission rp ON rp.role_id=r.id
JOIN permission p ON p.id=rp.permission_id AND p.status=1
JOIN iam_permissions ip ON ip.permission_code=CASE p.code
  WHEN 'asset.coin.balance.read' THEN 'asset:coin_balance:read'
  WHEN 'asset.coin.transaction.read' THEN 'asset:coin_transaction:read'
  WHEN 'asset.coin.correction.request' THEN 'asset:coin_correction:request'
  WHEN 'asset.coin.correction.approve' THEN 'asset:coin_correction:approve'
  WHEN 'asset.coin.correction.apply' THEN 'asset:coin_correction:apply'
  ELSE REPLACE(p.code,'.',':') END
WHERE r.status=1;

INSERT IGNORE INTO iam_user_role_bindings(user_id,role_id,scope_type,scope_id,status,granted_by)
SELECT au.user_id,ir.id,'global','',1,'migration-000047'
FROM admin_user_role aur JOIN admin_user au ON au.id=aur.admin_user_id
JOIN role r ON r.id=aur.role_id AND r.status=1
JOIN iam_roles ir ON ir.role_code=IF(r.name='super-admin','super_admin',CONCAT('legacy_',r.id))
WHERE au.status=1 AND au.is_delete=0 AND au.user_id IS NOT NULL AND LENGTH(au.user_id)<=64;

INSERT IGNORE INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r CROSS JOIN iam_permissions p WHERE r.role_code='super_admin';

INSERT IGNORE INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r JOIN iam_permissions p ON p.permission_code='iam:approval:approve' WHERE r.role_code='security_admin';

-- 有旧 RBAC 管理权的非超级管理员获得 IAM 管理与双人审批资格；无法凭自身完成授权。
INSERT IGNORE INTO iam_role_permissions(role_id,permission_id)
SELECT ir.id,ip.id FROM role r
JOIN iam_roles ir ON ir.role_code=CONCAT('legacy_',r.id)
JOIN role_permission rp ON rp.role_id=r.id
JOIN permission oldp ON oldp.id=rp.permission_id AND oldp.code='ops.rbac.manage' AND oldp.status=1
JOIN iam_permissions ip ON ip.permission_code IN ('iam:role:read','iam:role:grant','iam:approval:approve')
WHERE r.status=1;

-- 业务身份独立于管理员身份；资源归属仍由业务表和写入条件判定。
INSERT IGNORE INTO iam_roles(role_code,role_name,description,status,built_in) VALUES
('creator','视频创作者','仅管理本人投稿',1,1),('merchant','商家','仅管理本人店铺资源',1,1);
INSERT IGNORE INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r JOIN iam_permissions p ON
(r.role_code='creator' AND p.permission_code IN ('video:submission:create','video:submission:update')) OR
(r.role_code='merchant' AND p.permission_code IN ('mall:product:create','mall:product:update','mall:product:publish','mall:order:read','mall:order:ship'));
-- 历史投稿/商家身份在其本人下次写入时按唯一业务 ID 点查补齐，避免迁移扫描大表。

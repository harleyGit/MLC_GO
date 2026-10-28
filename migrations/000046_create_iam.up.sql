USE HG_MLC_DB;

-- 新 IAM 与现有 ops RBAC 并行；应用迁移前需确认旧 super-admin 绑定存在。
CREATE TABLE iam_roles (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '角色主键',
  role_code VARCHAR(64) NOT NULL COMMENT '稳定的角色编码，不随名称改变',
  role_name VARCHAR(100) NOT NULL COMMENT '管理端显示的角色名称',
  description VARCHAR(255) NOT NULL DEFAULT '' COMMENT '角色职责说明',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '角色状态：1 启用，0 停用',
  built_in TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否内置角色：1 是，0 否',
  version BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '权限策略版本，每次角色信息变更递增',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度，数据库会话时区',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(id), UNIQUE KEY uk_iam_role_code(role_code), KEY idx_iam_role_status_id(status,id)
) ENGINE=InnoDB COMMENT='IAM 角色';

CREATE TABLE iam_permissions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '权限主键',
  permission_code VARCHAR(128) NOT NULL COMMENT '业务域:资源:操作格式的权限编码',
  description VARCHAR(255) NOT NULL DEFAULT '' COMMENT '权限用途说明',
  PRIMARY KEY(id), UNIQUE KEY uk_iam_permission_code(permission_code)
) ENGINE=InnoDB COMMENT='IAM 权限目录';

CREATE TABLE iam_role_permissions (
  role_id BIGINT UNSIGNED NOT NULL COMMENT '角色主键',
  permission_id BIGINT UNSIGNED NOT NULL COMMENT '权限主键',
  PRIMARY KEY(role_id,permission_id), KEY idx_iam_rp_permission(permission_id),
  CONSTRAINT fk_iam_rp_role FOREIGN KEY(role_id) REFERENCES iam_roles(id),
  CONSTRAINT fk_iam_rp_permission FOREIGN KEY(permission_id) REFERENCES iam_permissions(id)
) ENGINE=InnoDB COMMENT='IAM 角色权限';

CREATE TABLE iam_organizations (
  id VARCHAR(64) NOT NULL COMMENT '业务组织标识',
  org_type VARCHAR(32) NOT NULL COMMENT '组织类型：shop 或 org',
  org_name VARCHAR(128) NOT NULL COMMENT '组织显示名称',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '组织状态：1 启用，0 停用',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(id), KEY idx_iam_org_type_status(org_type,status)
) ENGINE=InnoDB COMMENT='IAM 作用域组织登记';

CREATE TABLE iam_user_role_bindings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '绑定主键',
  user_id VARCHAR(64) NOT NULL COMMENT '现有 users.user_id 业务用户标识',
  role_id BIGINT UNSIGNED NOT NULL COMMENT 'IAM 角色主键',
  scope_type VARCHAR(32) NOT NULL COMMENT '作用域类型：global、shop 或 org',
  scope_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '作用域标识；global 时为空',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '授权状态：1 有效，0 已撤销',
  expires_at DATETIME(3) NULL COMMENT '可空失效时间，毫秒精度，数据库会话时区',
  granted_by VARCHAR(64) NOT NULL COMMENT '授权人业务用户标识',
  revoked_at DATETIME(3) NULL COMMENT '可空撤销时间，毫秒精度，数据库会话时区',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度，数据库会话时区',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(id), UNIQUE KEY uk_iam_binding(user_id,role_id,scope_type,scope_id),
  KEY idx_iam_binding_user_scope(user_id,scope_type,scope_id,status,role_id),
  KEY idx_iam_binding_user_page(user_id,status,id),
  KEY idx_iam_binding_role_scope(role_id,scope_type,scope_id,status),
  CONSTRAINT fk_iam_binding_role FOREIGN KEY(role_id) REFERENCES iam_roles(id)
) ENGINE=InnoDB COMMENT='IAM 用户作用域角色授权';

CREATE TABLE iam_audit_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '审计主键',
  actor_user_id VARCHAR(64) NOT NULL COMMENT '操作人业务用户标识',
  action VARCHAR(128) NOT NULL COMMENT '授权变更动作编码',
  target_type VARCHAR(64) NOT NULL COMMENT '目标类型',
  target_id VARCHAR(64) NOT NULL COMMENT '目标标识',
  request_id VARCHAR(64) NOT NULL COMMENT '请求关联标识',
  detail JSON NULL COMMENT '非敏感变更摘要；不记录令牌或凭据',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度，数据库会话时区',
  PRIMARY KEY(id), KEY idx_iam_audit_actor_time(actor_user_id,created_at),
  KEY idx_iam_audit_target_time(target_type,target_id,created_at)
) ENGINE=InnoDB COMMENT='IAM 追加式授权审计';

INSERT INTO iam_roles(role_code,role_name,description,status,built_in) VALUES
('super_admin','超级管理员','平台最高权限，仅供受控管理员使用',1,1),
('security_admin','安全管理员','管理角色、授权及审计',1,1),
('operator','运营人员','平台运营',1,1),
('content_reviewer','内容审核员','内容审核',1,1),
('crawler_operator','爬虫操作员','采集任务管理',1,1),
('ops_engineer','运维人员','系统观测与受控操作',1,1);

INSERT INTO iam_permissions(permission_code,description) VALUES
('iam:role:read','读取角色和授权'),('iam:role:grant','创建角色及管理授权'),
('video:submission:read','读取视频投稿'),('video:submission:update','修改视频投稿'),
('mall:product:create','创建商品'),('mall:product:update','修改商品'),
('crawler:task:execute','执行采集任务'),('ops:metrics:read','读取运维指标');

INSERT INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r CROSS JOIN iam_permissions p WHERE r.role_code='super_admin';
INSERT INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r JOIN iam_permissions p ON p.permission_code IN ('iam:role:read','iam:role:grant') WHERE r.role_code='security_admin';

-- 使用现有 super-admin 管理员绑定引导新权限，不写固定用户信息。
INSERT INTO iam_user_role_bindings(user_id,role_id,scope_type,scope_id,status,granted_by)
SELECT DISTINCT au.user_id,ir.id,'global','',1,'migration-000046'
FROM admin_user au JOIN admin_user_role aur ON aur.admin_user_id=au.id
JOIN role old_role ON old_role.id=aur.role_id
JOIN iam_roles ir ON ir.role_code='super_admin'
WHERE au.status=1 AND au.is_delete=0 AND old_role.name='super-admin' AND old_role.status=1;

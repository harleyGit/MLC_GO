-- 只新增模板与不可变版本，不改动既有 notification_inbox；迁移由发布流程执行。
CREATE TABLE notification_templates (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT '模板主键；游标按此字段倒序分页',
  template_code VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '全局唯一业务编码；创建后不可改',
  name VARCHAR(128) NOT NULL COMMENT '模板当前展示名称；编辑时同步更新',
  category VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '模板分类；固定通知分类枚举',
  scope_type VARCHAR(32) NOT NULL DEFAULT 'platform' COMMENT '模板业务作用域；本阶段仅允许平台作用域',
  scope_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '业务作用域标识；平台作用域固定为空',
  status VARCHAR(20) NOT NULL DEFAULT 'draft' COMMENT '模板状态；草稿 draft 或软删除 deleted',
  latest_version INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '最新草稿版本号；从 1 开始递增',
  published_version INT UNSIGNED DEFAULT NULL COMMENT '已发布版本；NULL 表示从未发布，本阶段不开放发布',
  created_by VARCHAR(128) NOT NULL COMMENT '创建管理员的可信 JWT 用户标识',
  updated_by VARCHAR(128) NOT NULL COMMENT '最近一次修改管理员的可信 JWT 用户标识',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒精度；数据库连接时区遵循现有工程约定',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间，毫秒精度；数据库连接时区遵循现有工程约定',
  UNIQUE KEY uk_notification_template_code (template_code),
  KEY idx_notification_template_page (id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE notification_template_versions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT '不可变版本行主键',
  template_id BIGINT UNSIGNED NOT NULL COMMENT '所属模板主键；与 version 共同唯一',
  version INT UNSIGNED NOT NULL COMMENT '模板版本号；从 1 开始递增，历史版本不更新',
  name VARCHAR(128) NOT NULL COMMENT '此版本的展示名称快照',
  category VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '此版本的通知分类快照',
  title_template VARCHAR(200) NOT NULL COMMENT '此版本纯文本标题模板',
  body_template TEXT NOT NULL COMMENT '此版本纯文本正文模板；应用层限制至多 2048 字符',
  variables_schema JSON NOT NULL COMMENT '变量名到类型、必填、长度及格式约束的 JSON 对象',
  action_type VARCHAR(32) NOT NULL DEFAULT 'none' COMMENT '内部路由动作枚举；none 表示不跳转',
  action_config JSON NOT NULL COMMENT '内部路由名称及参数的受控 JSON 对象',
  channels JSON NOT NULL COMMENT '站内与实时渠道枚举数组；本阶段不执行发送',
  priority TINYINT UNSIGNED NOT NULL DEFAULT 2 COMMENT '通知优先级，1 至 3，缺省 2',
  locale VARCHAR(16) NOT NULL DEFAULT 'zh-CN' COMMENT '语言区域；当前仅支持 zh-CN',
  status VARCHAR(20) NOT NULL DEFAULT 'draft' COMMENT '版本状态；本阶段只创建草稿 draft',
  created_by VARCHAR(128) NOT NULL COMMENT '创建本版本的可信管理员用户标识',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '版本创建时间，毫秒精度；数据库连接时区遵循现有工程约定',
  UNIQUE KEY uk_notification_template_version (template_id, version),
  CONSTRAINT fk_notification_template_version FOREIGN KEY (template_id) REFERENCES notification_templates(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 新权限仅分配给现有 super_admin；其他角色须经 IAM 授权，不自动开放发送能力。
START TRANSACTION;
INSERT INTO iam_permissions(permission_code,description) VALUES
('notification:template:read','查看系统通知模板'),
('notification:template:create','创建系统通知模板草稿'),
('notification:template:update','编辑系统通知模板并生成草稿版本'),
('notification:template:delete','软删除系统通知模板草稿');
INSERT INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r JOIN iam_permissions p
ON p.permission_code IN ('notification:template:read','notification:template:create','notification:template:update','notification:template:delete')
WHERE r.role_code='super_admin' AND r.status=1;
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

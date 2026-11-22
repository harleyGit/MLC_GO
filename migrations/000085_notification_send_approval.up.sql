-- 仅扩展有界明确用户任务；已存在任务保持原状态，默认立即处理。
-- 上线须先执行 000083、000084；MySQL 大表在线建索引/表结构变更须由发布系统评估锁等待。
ALTER TABLE notification_send_tasks
  ADD COLUMN scheduled_at DATETIME(3) NULL COMMENT 'UTC 计划投递时间；NULL 表示立即',
  ADD COLUMN approved_by VARCHAR(128) NULL COMMENT '另一位管理员的可信用户标识',
  ADD COLUMN approved_at DATETIME(3) NULL COMMENT 'UTC 审批时间',
  ADD KEY idx_notification_send_review (status,id);

CREATE TABLE notification_send_audit (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  task_id BIGINT UNSIGNED NOT NULL,
  action VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'created/approved/cancelled',
  actor_id VARCHAR(128) NOT NULL,
  request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '请求链路 ID，内部事务调用可为空',
  source_ip VARCHAR(45) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '请求直连 IP；不信任 X-Forwarded-For',
  template_version INT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_notification_send_audit_task (task_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='任务重要状态变更的不可变审计记录';

CREATE TABLE notification_template_audit (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  template_id BIGINT UNSIGNED NOT NULL,
  template_version INT UNSIGNED NOT NULL,
  action VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'published',
  actor_id VARCHAR(128) NOT NULL,
  request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  source_ip VARCHAR(45) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  content_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '发布时已验证模板快照 SHA-256',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_notification_template_audit (template_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='运营模板发布版本的不可变审计记录';

START TRANSACTION;
INSERT INTO iam_permissions(permission_code,description) VALUES
('notification:send:approve','审批另一名管理员创建的有界站内发送任务');
INSERT INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r JOIN iam_permissions p ON p.permission_code='notification:send:approve'
WHERE r.role_code='super_admin' AND r.status=1;
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

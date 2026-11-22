-- 有界指定用户发送队列；发布流程先执行迁移再启动 worker，不回填历史通知。
-- 千万级全站广播不得使用此表复制接收人；按 request_id 唯一键防止重复提交。
CREATE TABLE notification_send_tasks (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  template_id BIGINT UNSIGNED NOT NULL,
  template_version INT UNSIGNED NOT NULL,
  audience JSON NOT NULL COMMENT '至多 100 个指定用户 ID，创建时去重并排序',
  variables JSON NOT NULL COMMENT '已校验的任务级纯文本变量',
  created_by VARCHAR(128) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending' COMMENT 'pending/leased/done/failed/cancelled',
  next_index SMALLINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '下一名待发送用户的数组下标',
  lease_token CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  lease_until DATETIME(3) NOT NULL DEFAULT '1970-01-01 00:00:00.000',
  next_retry_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  attempts TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前接收人的连续失败次数',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_notification_send_request(request_id),
  KEY idx_notification_send_ready(status,next_retry_at,id),
  KEY idx_notification_send_lease(status,lease_until,id),
  KEY idx_notification_send_creator(created_by,status,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MySQL 持久化发送任务；本期仅有界站内信';

-- 新增运营发送/双人发布权限，只授予现有活动超级管理员；不复用本人试通知权限。
START TRANSACTION;
INSERT INTO iam_permissions(permission_code,description) VALUES
('notification:template:publish','复核并发布可用于运营发送的站内模板'),
('notification:send:create','创建至多一百名指定用户的站内发送任务'),
('notification:send:read','查询本人创建的站内发送任务'),
('notification:send:cancel','取消本人未完成的站内发送任务');
INSERT INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r JOIN iam_permissions p
ON p.permission_code IN ('notification:template:publish','notification:send:create','notification:send:read','notification:send:cancel')
WHERE r.role_code='super_admin' AND r.status=1;
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

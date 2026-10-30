-- 仅新增通知独立业务表；先迁移再发布代码，不自动发送外部通知。
-- 大量历史通知需要按接收人分片/归档；本表索引保证收件箱游标与未读筛选不全表扫描。
CREATE TABLE notification_inbox (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    event_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    receiver_id VARCHAR(128) NOT NULL,
    category VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    title VARCHAR(255) NOT NULL,
    content VARCHAR(2048) NOT NULL,
    target_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
    target_id VARCHAR(128) NOT NULL DEFAULT '',
    is_read TINYINT(1) NOT NULL DEFAULT 0,
    is_deleted TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_notify_event_receiver_category (event_id,receiver_id,category),
    KEY idx_notify_receiver_page (receiver_id,category,is_deleted,id DESC),
    KEY idx_notify_receiver_unread (receiver_id,is_deleted,is_read,category,id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户站内通知快照，IM 消息不存入此表';

CREATE TABLE notification_preferences (
    receiver_id VARCHAR(128) NOT NULL,
    preferences JSON NOT NULL,
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (receiver_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户可选通知偏好；系统通知不受偏好关闭';

-- 新能力默认不授予普通管理员；仅赋予已有 super_admin，按 IAM 审批流程授权其他角色。
START TRANSACTION;
INSERT INTO iam_permissions(permission_code,description) VALUES
('notification:channel:read','读取通知渠道状态'),
('notification:delivery:read','读取站内通知投递记录'),
('notification:channel:send','向当前管理员发送站内试通知');
INSERT INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r JOIN iam_permissions p ON p.permission_code IN
('notification:channel:read','notification:delivery:read','notification:channel:send') WHERE r.role_code='super_admin';
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

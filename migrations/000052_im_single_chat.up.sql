-- 仅单聊；历史以 (conversation_id,seq) 游标读取。上线前先评估分库/归档与分区键，禁止在线对亿级表直接做阻塞 DDL。
CREATE TABLE im_conversation (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_low VARCHAR(128) NOT NULL,
    user_high VARCHAR(128) NOT NULL,
    last_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_im_single_pair (user_low,user_high),
    KEY idx_im_conversation_updated (updated_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='单聊权威会话；一对用户仅一个会话';

CREATE TABLE im_conversation_member (
    conversation_id BIGINT UNSIGNED NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    last_read_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
    joined_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (conversation_id,user_id),
    KEY idx_im_member_user_page (user_id,conversation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会话成员、已读水位；查询必须按 JWT 用户过滤';

-- 先按会话水平分片/归档再扩大容量；单库的自增主键和会话单行锁不是亿级吞吐保证。
CREATE TABLE im_message (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    conversation_id BIGINT UNSIGNED NOT NULL,
    seq BIGINT UNSIGNED NOT NULL,
    sender_id VARCHAR(128) NOT NULL,
    client_msg_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    message_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    content JSON NOT NULL,
    content_hash BINARY(32) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_im_conversation_seq (conversation_id,seq),
    UNIQUE KEY uk_im_sender_client_id (sender_id,client_msg_id),
    KEY idx_im_message_conversation_page (conversation_id,seq,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='私信历史权威表；媒体只存私有引用';

-- 每台设备独立确认。设备 ID 必须由服务端随机签发并绑定 user_id；过期/注销须移除待投递任务。
CREATE TABLE im_device (
    id BINARY(16) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    credential_hash BINARY(32) NOT NULL,
    last_active_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    revoked_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_im_device_active (user_id,revoked_at,last_active_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='最多五台活跃设备；30天无活动失活';

-- 离线可靠性以 im_message 为权威，待投递是有界的每设备重试状态，不代表用户已读。
CREATE TABLE im_device_delivery (
    device_id BINARY(16) NOT NULL,
    message_id BIGINT UNSIGNED NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    attempt_count SMALLINT UNSIGNED NOT NULL DEFAULT 0,
    next_retry_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    lease_token BINARY(16) NULL,
    lease_until DATETIME(3) NULL,
    delivered_at DATETIME(3) NULL,
    PRIMARY KEY (device_id,message_id),
    KEY idx_im_delivery_retry (next_retry_at,lease_until,device_id,message_id),
    KEY idx_im_delivery_user (user_id,device_id,delivered_at,message_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='客户端每设备 ACK 前不可标记已送达';

-- 独立私有媒体桶，绝不存公开 URL；授权必须按会话成员和审核状态逐条检查。
CREATE TABLE im_media (
    id BINARY(16) NOT NULL,
    owner_id VARCHAR(128) NOT NULL,
    object_key VARCHAR(256) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    media_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    mime_type VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    byte_size BIGINT UNSIGNED NOT NULL,
    review_status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_im_media_object (object_key),
    KEY idx_im_media_owner (owner_id,created_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='私有 IM 附件归属与审核状态；无审核不得发送';

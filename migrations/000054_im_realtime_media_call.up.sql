-- 新增 IM 扩展表与投递索引；执行前需确认环境。时间沿用连接会话时区，服务端会话必须统一 UTC。
-- 不回填 000052；旧消息继续由授权历史接口补拉。禁止直接在亿级库执行未评审 DDL。
ALTER TABLE im_device_delivery ADD KEY idx_im_delivery_message(message_id,device_id), ALGORITHM=INPLACE, LOCK=NONE;
CREATE TABLE im_realtime_inbox (
    event_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'Outbox 稳定事件 ID',
    payload_hash BINARY(32) NOT NULL COMMENT '事件名称及载荷摘要；冲突不得静默去重',
    topic VARCHAR(249) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '首次接收 Kafka topic',
    partition_id INT NOT NULL COMMENT '首次接收分区编号',
    offset_id BIGINT NOT NULL COMMENT '首次接收 offset',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '首次接收时间，毫秒',
    PRIMARY KEY(event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='IM 独立消费去重，与实时任务同事务提交';

CREATE TABLE im_realtime_job (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '内部重试任务主键',
    event_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '事件幂等 ID',
    device_id BINARY(16) NOT NULL COMMENT '注册设备 ID；不是客户端自报标识',
    user_id VARCHAR(128) NOT NULL COMMENT '设备归属用户',
    message_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '消息 ID；通话事件为零',
    payload JSON NOT NULL COMMENT '只含事件/会话/消息或通话指针，不含私信正文或凭据',
    state TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0待尝试 1租约中 2完成 3耗尽待补拉',
    attempts SMALLINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '尝试次数，最多十二次',
    next_retry_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '下一次重试或租约接管时间，毫秒',
    lease_token BINARY(16) NULL COMMENT '租约栅栏；完成必须匹配本次 token',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒',
    PRIMARY KEY(id),
    UNIQUE KEY uk_im_job_event_device(event_id,device_id),
    KEY idx_im_job_due(state,next_retry_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='有界逐设备重试；发布成功不等于设备 ACK';

CREATE TABLE im_private_attachment (
    id BINARY(16) NOT NULL COMMENT '128 位随机媒体业务 ID',
    owner_id VARCHAR(128) NOT NULL COMMENT '上传者 JWT 用户',
    conversation_id BIGINT UNSIGNED NOT NULL COMMENT '上传时绑定单聊，禁止跨会话复用',
    object_key VARCHAR(256) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '隔离区随机对象 Key，不向消息写公开 URL',
    immutable_key VARCHAR(256) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '审核后只读对象 Key，客户端无写权限',
    media_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '媒体类型：image图片/video视频/audio音频/file文件',
    mime_type VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '服务端允许的 MIME，审核器必须验证真实格式',
    byte_size BIGINT UNSIGNED NOT NULL COMMENT '申请大小，字节；审核须验证实际一致',
    sha256 BINARY(32) NOT NULL COMMENT '申请 SHA256；审核器必须计算实际摘要比对',
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'uploading' COMMENT '附件状态：uploading上传中/verifying审核中/ready可用/rejected拒绝/deleted已撤销',
    width INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '审核得到的像素宽度，非图片视频为零',
    height INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '审核得到的像素高度，非图片视频为零',
    duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '审核得到的时长，毫秒；静态文件为零',
    upload_expires_at DATETIME(3) NOT NULL COMMENT '上传凭证有效截止，毫秒',
    next_check_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '后台审核重试时间，毫秒',
    lease_token BINARY(16) NULL COMMENT '审核工作租约栅栏',
    attempts SMALLINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '审核调用次数，最多十二次',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒',
    PRIMARY KEY(id),
    UNIQUE KEY uk_im_attachment_object(object_key),
    KEY idx_im_attachment_owner(owner_id,created_at,id),
    KEY idx_im_attachment_review(state,next_check_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='私有附件；审核器复制到不可变 Key 后才能 ready';

CREATE TABLE im_message_attachment (
    message_id BIGINT UNSIGNED NOT NULL COMMENT '已提交消息 ID',
    attachment_id BINARY(16) NOT NULL COMMENT '已审核媒体 ID',
    conversation_id BIGINT UNSIGNED NOT NULL COMMENT '绑定会话；逐次读取必须校验成员',
    PRIMARY KEY(message_id,attachment_id),
    KEY idx_im_attachment_message(attachment_id,conversation_id,message_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='媒体与消息原子关联，不通过 JSON 扫描授权';

CREATE TABLE im_call_session (
    id BINARY(16) NOT NULL COMMENT '随机呼叫业务 ID，同时参与服务端房间名',
    conversation_id BIGINT UNSIGNED NOT NULL COMMENT '单聊会话 ID',
    caller_id VARCHAR(128) NOT NULL COMMENT '主叫用户',
    callee_id VARCHAR(128) NOT NULL COMMENT '被叫用户',
    request_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '主叫幂等请求 ID',
    caller_device BINARY(16) NOT NULL COMMENT '发起呼叫设备',
    callee_device BINARY(16) NULL COMMENT '唯一成功接听设备',
    media_type VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '通话类型：audio语音/video视频',
    room_sid VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT 'LiveKit 房间实例 SID，防止旧房间回调关闭重建实例',
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'ringing' COMMENT '呼叫状态：ringing振铃/accepted已接听/ended已结束/rejected已拒绝/cancelled已取消/timeout已超时',
    version BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态变更版本；客户端以此去重排序',
    expires_at DATETIME(3) NOT NULL COMMENT '振铃截止或通话硬截止时间，毫秒',
    cleanup_at DATETIME(3) NULL COMMENT '结束后房间清理重试时间，需覆盖所有未过期令牌',
    cleanup_until DATETIME(3) NULL COMMENT '最后已签发令牌失效后额外缓冲截止',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间，毫秒',
    PRIMARY KEY(id),
    UNIQUE KEY uk_im_call_request(caller_id,request_id),
    KEY idx_im_call_expiry(state,expires_at,id),
    KEY idx_im_call_cleanup(cleanup_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='单聊呼叫状态；接听使用行锁保证单设备胜出';

CREATE TABLE im_call_busy (
    user_id VARCHAR(128) NOT NULL COMMENT '用户唯一忙线占位',
    call_id BINARY(16) NOT NULL COMMENT '占用中的呼叫 ID；释放需匹配',
    PRIMARY KEY(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='双方固定顺序占位，同事务建立/释放忙线';

CREATE TABLE im_call_webhook (
    event_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'LiveKit 回调事件 ID',
    payload_hash BINARY(32) NOT NULL COMMENT '验签后原始载荷摘要，防止同 ID 不同内容',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '接收时间，毫秒，沿用 UTC 会话时区',
    PRIMARY KEY(event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='通话回调幂等记录，与通话终态及 Outbox 同事务';

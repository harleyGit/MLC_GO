-- 先部署迁移再部署新的鉴权/设备注册代码；旧设备无 session_id，达到上限时仍须人工退出。
-- 百万级设备表上线前评估在线 DDL、锁等待、回填与存储压力，禁止在业务高峰执行。
ALTER TABLE im_device
    ADD COLUMN session_id BINARY(16) NULL COMMENT '登录会话随机 ID；旧记录为 NULL，不可自动踢完整会话',
    ADD COLUMN created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '设备首次注册时间；旧记录迁移时为相同时间',
    ADD COLUMN forced_logout_at DATETIME(3) NULL COMMENT '仅设备超额替换时标记该登录会话被强制下线',
    ADD KEY idx_im_device_oldest (user_id,revoked_at,created_at,id),
    ADD KEY idx_im_device_session_logout (user_id,session_id,forced_logout_at);

-- 回滚前必须停止新版本服务；下线后无法再将 IM 设备与登录会话关联。
ALTER TABLE im_device
    DROP INDEX idx_im_device_oldest,
    DROP INDEX idx_im_device_session_logout,
    DROP COLUMN session_id,
    DROP COLUMN forced_logout_at,
    DROP COLUMN created_at;

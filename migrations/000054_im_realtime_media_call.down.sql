-- 删除业务数据的破坏性回滚，仅获授权的隔离空库使用。
DROP TABLE IF EXISTS im_call_webhook;
DROP TABLE IF EXISTS im_call_busy;
DROP TABLE IF EXISTS im_call_session;
DROP TABLE IF EXISTS im_message_attachment;
DROP TABLE IF EXISTS im_private_attachment;
DROP TABLE IF EXISTS im_realtime_job;
DROP TABLE IF EXISTS im_realtime_inbox;
ALTER TABLE im_device_delivery DROP INDEX idx_im_delivery_message;

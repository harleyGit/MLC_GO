-- 回滚仅恢复旧列表形态；已物理删除的历史消息不可恢复，必须依赖备份。
ALTER TABLE im_realtime_job DROP KEY idx_im_job_message, ALGORITHM=INPLACE, LOCK=NONE;
ALTER TABLE im_message DROP KEY idx_im_message_retention, ALGORITHM=INPLACE, LOCK=NONE;
ALTER TABLE im_conversation_member DROP COLUMN deleted_at, DROP COLUMN deleted_seq, DROP COLUMN muted, ALGORITHM=INPLACE, LOCK=NONE;

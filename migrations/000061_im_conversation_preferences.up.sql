-- 本人会话隐藏与免打扰不改变成员关系；历史清理按会话与创建时间索引分批执行。
-- im_message / im_realtime_job 大表新增索引须先评估容量与在线 DDL 能力，不自动执行迁移。
ALTER TABLE im_conversation_member
    ADD COLUMN deleted_at DATETIME(3) NULL COMMENT '本人删除会话的时间；NULL表示可见',
    ADD COLUMN deleted_seq BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '本人隐藏会话时的消息水位；新消息可使会话重新出现',
    ADD COLUMN muted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '本人新消息免打扰，0关闭1开启',
    ALGORITHM=INPLACE, LOCK=NONE;
ALTER TABLE im_message ADD KEY idx_im_message_retention (conversation_id,created_at,id), ALGORITHM=INPLACE, LOCK=NONE;
ALTER TABLE im_realtime_job ADD KEY idx_im_job_message (message_id,id), ALGORITHM=INPLACE, LOCK=NONE;

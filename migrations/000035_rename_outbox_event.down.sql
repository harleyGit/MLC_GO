-- 仅恢复旧表名，保留迁移前后写入的事件记录及租约数据。
-- 回退前停止新版 Outbox 写入方及 dispatcher，回退后再启动旧版；目标表必须不存在。
RENAME TABLE `outbox_event` TO `outbox_events`;

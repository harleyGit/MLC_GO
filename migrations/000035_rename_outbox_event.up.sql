-- 仅重命名，保留已有列、索引、事件记录及租约数据。
-- 切换前停止旧版 Outbox 写入方及 dispatcher，迁移后再启动新版；目标表必须不存在。
RENAME TABLE `outbox_events` TO `outbox_event`;

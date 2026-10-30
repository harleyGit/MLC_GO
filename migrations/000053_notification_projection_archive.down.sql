-- 仅空库隔离环境可执行；生产删除计数、归档及去重墓碑会造成历史丢失和事件重复。
DROP TABLE notification_inbox_archive;
DROP TABLE notification_event_ledger;
DROP TABLE notification_unread_shard;
DROP TABLE notification_counter_control;

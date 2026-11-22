-- 仅在不存在 delivery_mode=kafka 的任务、Kafka 出站积压及在途事件时，才允许人工回退。
-- 此脚本不自动清理 Kafka 事件，也不转换任务所有权。
ALTER TABLE notification_send_tasks
  DROP KEY idx_notification_send_mode_lease,
  DROP KEY idx_notification_send_mode_ready,
  DROP COLUMN dispatch_outbox_id,
  DROP COLUMN dispatch_generation,
  DROP COLUMN delivery_mode;

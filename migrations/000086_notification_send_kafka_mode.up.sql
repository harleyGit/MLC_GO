-- 先迁移、再部署引用 delivery_mode 的代码；已有任务默认由原 MySQL 工作者继续处理。
-- 在实际任务量大的环境评估在线建索引的锁等待及回退窗口，不在迁移中修改任务状态。
ALTER TABLE notification_send_tasks
  ADD COLUMN delivery_mode VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'mysql'
    COMMENT 'mysql/kafka；按任务固定，禁止双工作者认领',
  ADD COLUMN dispatch_generation BIGINT UNSIGNED NOT NULL DEFAULT 0
    COMMENT '每次 Kafka 到期入队递增，旧代次不可领取',
  ADD COLUMN dispatch_outbox_id BIGINT UNSIGNED NOT NULL DEFAULT 0
    COMMENT '当前代次的 Outbox 主键；pending 时可为 0',
  ADD KEY idx_notification_send_mode_ready (delivery_mode,status,next_retry_at,id),
  ADD KEY idx_notification_send_mode_lease (delivery_mode,status,lease_until,id);

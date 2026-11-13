-- 增量引入审核、封存与可恢复清理；不回填历史对象，不自动将现有 pending 转为 ready。
ALTER TABLE media_assets
  ADD COLUMN sealed_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL COMMENT '可信审核器写入的不可变封存对象键；空表示未封存',
  ADD COLUMN width INT UNSIGNED DEFAULT NULL COMMENT '可信审核器探测的宽度，单位像素；非图片/视频为空',
  ADD COLUMN height INT UNSIGNED DEFAULT NULL COMMENT '可信审核器探测的高度，单位像素；非图片/视频为空',
  ADD COLUMN duration_ms BIGINT UNSIGNED DEFAULT NULL COMMENT '可信审核器探测的音视频时长，单位毫秒；其他类型为空';

ALTER TABLE media_upload_sessions
  ADD COLUMN attempts INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '可信审核器处理尝试次数，最多 12 次',
  ADD COLUMN next_check_at DATETIME(3) DEFAULT NULL COMMENT '下次审核调度时间，UTC 毫秒；待上传时为空',
  ADD COLUMN lease_token BINARY(16) DEFAULT NULL COMMENT '审核租约随机令牌；防止过期 worker 写入结果',
  ADD KEY idx_media_review (status, next_check_at, upload_id);

CREATE TABLE media_delete_jobs (
  job_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '单个对象删除任务主键',
  asset_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '归属的媒体资源 ID',
  bucket VARCHAR(128) NOT NULL COMMENT '对象所在桶，必须等于服务端隔离桶配置',
  object_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '仅允许 quarantine/ 或 sealed/ 前缀',
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/leased/done/failed',
  attempts INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'S3 删除尝试次数，超过 12 次标记 failed 待人工处理',
  execute_after DATETIME(3) NOT NULL COMMENT '删除保护期或重试到期时间，UTC 毫秒',
  lease_token BINARY(16) DEFAULT NULL COMMENT '本轮删除租约令牌，过期后允许恢复',
  finalized_at DATETIME(3) DEFAULT NULL COMMENT '该删除结果已完成资源状态核对的时间，UTC 毫秒；空表示待恢复',
  updated_at DATETIME(3) NOT NULL COMMENT '最近状态更新时间，UTC 毫秒',
  PRIMARY KEY (job_id),
  UNIQUE KEY uk_media_delete_object (bucket, object_key),
  KEY idx_media_delete_due (status, execute_after, job_id),
  KEY idx_media_delete_asset (asset_id, status),
  KEY idx_media_delete_repair (status, finalized_at, job_id)
) ENGINE=InnoDB;

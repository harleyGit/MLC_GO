-- 衍生计数只在已有收件箱为空时就绪；已有数据须在暂停写入的隔离窗口按用户重建并核对后切换。
CREATE TABLE notification_counter_control (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
  ready TINYINT(1) NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='通知计数切换状态；未就绪时仍读取 MySQL 收件箱';

CREATE TABLE notification_unread_shard (
  receiver_id VARCHAR(128) NOT NULL,
  category VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  shard TINYINT UNSIGNED NOT NULL,
  unread_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY(receiver_id,category,shard)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户分类16分片未读计数；可从权威收件箱重建';

-- 保留归档后事件去重键；按接收用户稳定路由分片，不删除或重置才能抵御 Kafka 历史重放。
CREATE TABLE notification_event_ledger (
  archive_shard TINYINT UNSIGNED NOT NULL,
  event_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  receiver_id VARCHAR(128) NOT NULL,
  category VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY(archive_shard,event_id,receiver_id,category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
PARTITION BY HASH(archive_shard) PARTITIONS 16;

-- 历史按 receiver 哈希物理分区，接口仍以活跃 notification_inbox 为权威；归档不得把未读记录迁走。
CREATE TABLE notification_inbox_archive (
  archive_shard TINYINT UNSIGNED NOT NULL,
  id BIGINT UNSIGNED NOT NULL,
  event_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  receiver_id VARCHAR(128) NOT NULL,
  category VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  title VARCHAR(255) NOT NULL,
  content VARCHAR(2048) NOT NULL,
  target_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  target_id VARCHAR(128) NOT NULL DEFAULT '',
  is_read TINYINT(1) NOT NULL,
  is_deleted TINYINT(1) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  archived_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY(archive_shard,id),
  KEY idx_notify_archive_user_page(receiver_id,category,id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
PARTITION BY HASH(archive_shard) PARTITIONS 16;

-- 常数时间存在性检查；不对任何历史通知执行全表 backfill。
INSERT INTO notification_counter_control(id,ready)
SELECT 1, CASE WHEN EXISTS(SELECT id FROM notification_inbox LIMIT 1) THEN 0 ELSE 1 END;

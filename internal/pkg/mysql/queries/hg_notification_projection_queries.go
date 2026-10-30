package SQLQueriesPackage

import "strings"

// 16 分片按 notification_inbox.id 低四位路由；MySQL 与 Go 必须使用同一取模规则。
const (
	HGNotificationCounterReadySQL      = `SELECT ready FROM notification_counter_control WHERE id=1`
	HGNotificationShardUnreadSQL       = `SELECT category,SUM(unread_count) FROM notification_unread_shard WHERE receiver_id=? GROUP BY category`
	HGNotificationCounterIncreaseSQL   = `INSERT INTO notification_unread_shard(receiver_id,category,shard,unread_count) VALUES (?,?,?,1) ON DUPLICATE KEY UPDATE unread_count=unread_count+1`
	HGNotificationCounterDecreaseSQL   = `UPDATE notification_unread_shard SET unread_count=unread_count-? WHERE receiver_id=? AND category=? AND shard=? AND unread_count>=?`
	HGNotificationSelectUnreadBatchSQL = `SELECT id FROM notification_inbox WHERE receiver_id=? AND category=? AND is_deleted=0 AND is_read=0 ORDER BY id DESC LIMIT 500 FOR UPDATE`
	HGNotificationSelectDeleteSQL      = `SELECT category,is_read FROM notification_inbox WHERE receiver_id=? AND id=? AND is_deleted=0 FOR UPDATE`
	HGNotificationLedgerInsertSQL      = `INSERT INTO notification_event_ledger(archive_shard,event_id,receiver_id,category) VALUES (?,?,?,?)`
	HGNotificationArchiveWindowSQL     = `SELECT id,event_id,receiver_id,category,title,content,target_type,target_id,is_read,is_deleted,created_at FROM notification_inbox WHERE id>? AND id<=? AND created_at<? AND is_deleted=1 ORDER BY id ASC LIMIT 100 FOR UPDATE`
	HGNotificationArchiveInsertSQL     = `INSERT INTO notification_inbox_archive(archive_shard,id,event_id,receiver_id,category,title,content,target_type,target_id,is_read,is_deleted,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE id=id`
	HGNotificationArchiveDeleteSQL     = `DELETE FROM notification_inbox WHERE id=? AND is_deleted=1`
	HGNotificationRebuildPageSQL       = `SELECT id,category FROM notification_inbox WHERE receiver_id=? AND is_deleted=0 AND is_read=0 AND id>? ORDER BY id ASC LIMIT 501`
	HGNotificationDeleteUserShardsSQL  = `DELETE FROM notification_unread_shard WHERE receiver_id=?`
	HGNotificationSetUserShardSQL      = `INSERT INTO notification_unread_shard(receiver_id,category,shard,unread_count) VALUES (?,?,?,?)`
)

// HGNotificationSelectUnreadIDsSQL 以索引与 IN 占位符锁住至多 50 条本人的未读行。
func HGNotificationSelectUnreadIDsSQL(count int) string {
	if count < 1 || count > 50 {
		return ""
	}
	return `SELECT id,category FROM notification_inbox WHERE receiver_id=? AND is_deleted=0 AND is_read=0 AND id IN (` + strings.TrimSuffix(strings.Repeat("?,", count), ",") + `) ORDER BY id ASC FOR UPDATE`
}

// HGNotificationSetReadSelectedSQL 修改本事务已锁定的至多 500 条行；不存在深 OFFSET。
func HGNotificationSetReadSelectedSQL(count int) string {
	if count < 1 || count > 500 {
		return ""
	}
	return `UPDATE notification_inbox SET is_read=1 WHERE receiver_id=? AND is_deleted=0 AND is_read=0 AND id IN (` + strings.TrimSuffix(strings.Repeat("?,", count), ",") + `)`
}

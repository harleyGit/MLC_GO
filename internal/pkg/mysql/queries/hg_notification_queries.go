package SQLQueriesPackage

import "strings"

// 站内通知只按 receiver_id 与自增 ID 游标读取；依赖 idx_notify_receiver_page，最多读取 51 条。
const HGNotificationPageSQL = `SELECT id,category,title,content,target_type,target_id,is_read,created_at FROM notification_inbox WHERE receiver_id=? AND category=? AND is_deleted=0 AND id<? ORDER BY id DESC LIMIT ?`

// 未读页使用 idx_notify_receiver_unread 的接收人/状态前缀；不进行深 OFFSET。
const HGNotificationUnreadPageSQL = `SELECT id,category,title,content,target_type,target_id,is_read,created_at FROM notification_inbox WHERE receiver_id=? AND is_deleted=0 AND is_read=0 AND category=? AND id<? ORDER BY id DESC LIMIT ?`

// 汇总只扫描当前用户的未读索引范围；高未读量用户需后续用可重建的分片计数替代实时 COUNT。
const HGNotificationUnreadSQL = `SELECT category,COUNT(*) FROM notification_inbox WHERE receiver_id=? AND is_deleted=0 AND is_read=0 GROUP BY category`

// HGNotificationReadIDsSQL 集中拼接受限 1-50 个占位符；用户 ID 与通知 ID 仍全部通过参数绑定。
func HGNotificationReadIDsSQL(count int) string {
	if count < 1 || count > 50 {
		return ""
	}
	return `UPDATE notification_inbox SET is_read=1 WHERE receiver_id=? AND is_deleted=0 AND is_read=0 AND id IN (` + strings.TrimSuffix(strings.Repeat("?,", count), ",") + `)`
}

const HGNotificationDeleteSQL = `UPDATE notification_inbox SET is_deleted=1 WHERE receiver_id=? AND id=? AND is_deleted=0`

// 批量全部已读单次最多 500 行，依赖 idx_notify_receiver_page；用 id 上界防止持续写入造成无限循环。
const HGNotificationReadBatchSQL = `UPDATE notification_inbox SET is_read=1 WHERE receiver_id=? AND category=? AND is_deleted=0 AND is_read=0 AND id<=? ORDER BY id DESC LIMIT 500`
const HGNotificationHighWaterSQL = `SELECT COALESCE(MAX(id),0) FROM notification_inbox WHERE receiver_id=? AND category=? AND is_deleted=0`

const HGNotificationPreferencesSQL = `SELECT preferences FROM notification_preferences WHERE receiver_id=?`
const HGNotificationPreferencesUpsertSQL = `INSERT INTO notification_preferences(receiver_id,preferences) VALUES (?,?) ON DUPLICATE KEY UPDATE preferences=VALUES(preferences)`

// 事件/用户/分类唯一键保证调用方重试时站内信只落一份；必须提供稳定的上游 event_id。
const HGNotificationInsertSQL = `INSERT INTO notification_inbox(event_id,receiver_id,category,title,content,target_type,target_id) VALUES (?,?,?,?,?,?,?)`

// 运维投递列表仅按主键最近 20 条检查，不跨全表做全局计数；生产大表需按时间/接收人分片观测。
const HGNotificationDeliveryPageSQL = `SELECT id,event_id,receiver_id,category,created_at FROM notification_inbox WHERE id<? ORDER BY id DESC LIMIT ?`

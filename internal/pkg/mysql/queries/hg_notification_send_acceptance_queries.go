//go:build notification_acceptance

package SQLQueriesPackage

// 仅本机 debug 隔离验收使用；所有读写均以专用测试身份或新建主键限界，不触及共享用户。
const HGNotificationAcceptanceInsertUserSQL = `INSERT INTO users(user_id,user_name,password_hash,salt) VALUES (?,?,?,?)`
const HGNotificationAcceptanceInsertAdminSQL = `INSERT INTO admin_user(user_id,name,nick_name,mobile,password,update_at) VALUES (?,?,?,?,?,UTC_TIMESTAMP())`
const HGNotificationAcceptancePendingSQL = `SELECT id FROM notification_send_tasks WHERE delivery_mode='kafka' AND status IN ('pending','queued','leased','pending_approval') LIMIT 1`
const HGNotificationAcceptanceTaskStateSQL = `SELECT status,next_index,dispatch_generation,dispatch_outbox_id FROM notification_send_tasks WHERE id=? AND created_by=?`
const HGNotificationAcceptanceInboxSQL = `SELECT id,is_read FROM notification_inbox WHERE receiver_id=? AND event_id=? AND category='system'`
const HGNotificationAcceptanceOutboxStatusSQL = `SELECT status FROM outbox_event WHERE id=? AND topic='mlc.notification.send.ready.v1'`
const HGNotificationAcceptanceOutboxPayloadSQL = `SELECT payload FROM outbox_event WHERE id=? AND topic='mlc.notification.send.ready.v1' AND status='pending'`
const HGNotificationAcceptanceOutboxPublishSQL = `UPDATE outbox_event SET status='published',published_at=UTC_TIMESTAMP(),updated_at=UTC_TIMESTAMP() WHERE id=? AND topic='mlc.notification.send.ready.v1' AND status='pending'`
const HGNotificationAcceptanceDeleteInboxSQL = `DELETE FROM notification_inbox WHERE id=? AND receiver_id=? AND event_id=?`
const HGNotificationAcceptanceDeleteLedgerSQL = `DELETE FROM notification_event_ledger WHERE receiver_id=? AND event_id=? AND category='system'`
const HGNotificationAcceptanceDeleteAuditSQL = `DELETE FROM notification_send_audit WHERE task_id=?`
const HGNotificationAcceptanceDeleteTaskSQL = `DELETE FROM notification_send_tasks WHERE id=? AND created_by=?`
const HGNotificationAcceptanceDeleteOutboxSQL = `DELETE FROM outbox_event WHERE id=? AND topic='mlc.notification.send.ready.v1'`
const HGNotificationAcceptanceDeleteTemplateAuditSQL = `DELETE FROM notification_template_audit WHERE template_id=?`
const HGNotificationAcceptanceDeleteVersionSQL = `DELETE FROM notification_template_versions WHERE template_id=?`
const HGNotificationAcceptanceDeleteTemplateSQL = `DELETE FROM notification_templates WHERE id=? AND template_code=?`
const HGNotificationAcceptanceDeleteAdminSQL = `DELETE FROM admin_user WHERE user_id=? AND mobile=?`
const HGNotificationAcceptanceDeleteUserSQL = `DELETE FROM users WHERE user_id=? AND user_name=?`
const HGNotificationAcceptanceDeleteCounterSQL = `DELETE FROM notification_unread_shard WHERE receiver_id=? AND category='system' AND unread_count=0`

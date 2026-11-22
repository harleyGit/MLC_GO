package SQLQueriesPackage

import "strings"

// HGNotificationSendTemplateSQL 按主键及版本唯一键查询已发布的不可变版本；不允许最新草稿代替发布版。
const HGNotificationSendTemplateSQL = `SELECT v.category,v.title_template,v.body_template,v.variables_schema,v.action_type,v.action_config,v.channels
FROM notification_templates t JOIN notification_template_versions v ON v.template_id=t.id AND v.version=?
WHERE t.id=? AND t.status='published' AND t.published_version=v.version AND v.status='published'`

// HGNotificationSendBoundVersionSQL 任务绑定历史发布版；编辑并发布新版不改变已创建任务内容。
const HGNotificationSendBoundVersionSQL = `SELECT v.category,v.title_template,v.body_template,v.variables_schema,v.action_type,v.action_config,v.channels
FROM notification_templates t JOIN notification_template_versions v ON v.template_id=t.id AND v.version=?
WHERE t.id=? AND t.status='published' AND v.status='published'`

// HGNotificationSendPublishSQL 锁定模板主键后仅发布当前草稿；创建者不得作为复核人。
const HGNotificationSendPublishLockSQL = `SELECT t.latest_version,t.status,t.template_code,v.created_by,v.category,v.title_template,v.body_template,v.variables_schema,v.action_type,v.action_config,v.channels
FROM notification_templates t JOIN notification_template_versions v ON v.template_id=t.id AND v.version=t.latest_version
WHERE t.id=? FOR UPDATE`
const HGNotificationSendPublishVersionSQL = `UPDATE notification_template_versions SET status='published' WHERE template_id=? AND version=? AND status='draft'`

const HGNotificationSendInsertSQL = `INSERT INTO notification_send_tasks(request_id,request_hash,template_id,template_version,audience,variables,created_by,next_retry_at) VALUES (?,?,?,?,?,?,?,UTC_TIMESTAMP(3))`
const HGNotificationSendByRequestSQL = `SELECT id,request_hash FROM notification_send_tasks WHERE request_id=?`

// 创建时串行化同一管理员的短事务，不让并发请求同时越过未完成任务上限；依赖 admin_user.idx_user_id。
const HGNotificationSendAdminLockSQL = `SELECT id FROM admin_user WHERE user_id=? AND status=1 AND is_delete=0 ORDER BY id LIMIT 1 FOR UPDATE`

// 限量子查询只触碰至多 20 条 idx_notification_send_creator 索引记录，不能用无界 COUNT。
const HGNotificationSendPendingLimitSQL = `SELECT COUNT(*) FROM (SELECT id FROM notification_send_tasks WHERE created_by=? AND status IN ('pending','leased') LIMIT 20) AS pending_tasks`
const HGNotificationSendDetailSQL = `SELECT id,template_id,template_version,audience,status,next_index,attempts,created_at FROM notification_send_tasks WHERE id=? AND created_by=?`
const HGNotificationSendCancelSQL = `UPDATE notification_send_tasks SET status='cancelled' WHERE id=? AND created_by=? AND status IN ('pending','leased')`

// 两条候选查询分别走 ready 与 lease 索引；SKIP LOCKED 保证多 Pod 并行抢占且不在事务内投递。
const HGNotificationSendClaimReadySQL = `SELECT id,template_id,template_version,audience,variables,next_index,attempts
FROM notification_send_tasks WHERE status='pending' AND next_retry_at<=UTC_TIMESTAMP(3) ORDER BY next_retry_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`
const HGNotificationSendClaimExpiredSQL = `SELECT id,template_id,template_version,audience,variables,next_index,attempts
FROM notification_send_tasks WHERE status='leased' AND lease_until<=UTC_TIMESTAMP(3) ORDER BY lease_until,id LIMIT 1 FOR UPDATE SKIP LOCKED`
const HGNotificationSendLeaseSQL = `UPDATE notification_send_tasks SET status='leased',lease_token=?,lease_until=DATE_ADD(UTC_TIMESTAMP(3),INTERVAL 30 SECOND) WHERE id=?`
const HGNotificationSendAdvanceSQL = `UPDATE notification_send_tasks SET next_index=?,status=?,attempts=0,lease_token='',lease_until='1970-01-01 00:00:00.000' WHERE id=? AND status='leased' AND lease_token=? AND next_index=?`
const HGNotificationSendRetrySQL = `UPDATE notification_send_tasks SET status=?,attempts=?,next_retry_at=DATE_ADD(UTC_TIMESTAMP(3),INTERVAL ? SECOND),lease_token='',lease_until='1970-01-01 00:00:00.000' WHERE id=? AND status='leased' AND lease_token=? AND next_index=?`
const HGNotificationSendCorruptSQL = `UPDATE notification_send_tasks SET status='failed',lease_token='',lease_until='1970-01-01 00:00:00.000' WHERE id=? AND status='leased' AND lease_token=?`

// 一次最多 100 个主键候选；users.user_id 唯一索引，禁止扫描用户目录做分群。
// 查询片段由固定占位符拼接，用户 ID 始终通过参数绑定。
const HGNotificationSendUsersPrefixSQL = `SELECT user_id FROM users WHERE user_id IN (`
// 投递时按唯一 user_id 再确认用户仍存在；不可把创建时的身份快照当作永久可达凭据。
const HGNotificationSendUserExistsSQL = `SELECT 1 FROM users WHERE user_id=? LIMIT 1`

// HGNotificationSendUsersSQL 只产生 1-100 个占位符；调用方传入绑定参数，禁止拼入原始用户 ID。
func HGNotificationSendUsersSQL(count int) string {
	if count < 1 || count > 100 {
		return ""
	}
	return HGNotificationSendUsersPrefixSQL + strings.TrimSuffix(strings.Repeat("?,", count), ",") + ")"
}

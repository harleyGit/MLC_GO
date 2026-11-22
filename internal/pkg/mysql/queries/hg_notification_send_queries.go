package SQLQueriesPackage

import "strings"

// 000083-000085 上线只读校验：仅统计当前 schema 的列/索引/表和权限元数据，不扫描任务或审计数据。
const HGNotificationSendSchemaColumnsSQL = `SELECT COUNT(*) FROM information_schema.columns
WHERE table_schema=DATABASE() AND table_name='notification_send_tasks' AND column_name IN ('scheduled_at','approved_by','approved_at')`
const HGNotificationSendSchemaIndexesSQL = `SELECT COUNT(DISTINCT CONCAT(table_name,'/',index_name)) FROM information_schema.statistics
WHERE table_schema=DATABASE() AND ((table_name='notification_send_tasks' AND index_name IN ('idx_notification_send_mine','idx_notification_send_review'))
OR (table_name='notification_templates' AND index_name='idx_notification_template_published'))`
const HGNotificationSendSchemaAuditTablesSQL = `SELECT COUNT(*) FROM information_schema.tables
WHERE table_schema=DATABASE() AND table_name IN ('notification_send_audit','notification_template_audit')`
const HGNotificationSendSchemaPermissionSQL = `SELECT COUNT(*) FROM iam_permissions WHERE permission_code='notification:send:approve'`

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
const HGNotificationTemplateAuditSQL = `INSERT INTO notification_template_audit(template_id,template_version,action,actor_id,request_id,source_ip,content_hash) VALUES (?,?,?,?,?,?,?)`

const HGNotificationSendInsertSQL = `INSERT INTO notification_send_tasks(request_id,request_hash,template_id,template_version,audience,variables,created_by,next_retry_at) VALUES (?,?,?,?,?,?,?,UTC_TIMESTAMP(3))`

// 营销类任务必须经另一管理员审批；定时任务使用 UTC 到期时间且审批前不可被工作者领取。
const HGNotificationSendScheduledInsertSQL = `INSERT INTO notification_send_tasks(request_id,request_hash,template_id,template_version,audience,variables,created_by,status,scheduled_at,next_retry_at) VALUES (?,?,?,?,?,?,?,'pending_approval',?,COALESCE(?,UTC_TIMESTAMP(3)))`
const HGNotificationSendByRequestSQL = `SELECT id,request_hash FROM notification_send_tasks WHERE request_id=?`

// 创建时串行化同一管理员的短事务，不让并发请求同时越过未完成任务上限；依赖 admin_user.idx_user_id。
const HGNotificationSendAdminLockSQL = `SELECT id FROM admin_user WHERE user_id=? AND status=1 AND is_delete=0 ORDER BY id LIMIT 1 FOR UPDATE`

// 限量子查询只触碰至多 20 条 idx_notification_send_creator 索引记录，不能用无界 COUNT。
const HGNotificationSendPendingLimitSQL = `SELECT COUNT(*) FROM (SELECT id FROM notification_send_tasks WHERE created_by=? AND status IN ('pending','leased','pending_approval') LIMIT 20) AS pending_tasks`
const HGNotificationSendDetailSQL = `SELECT id,template_id,template_version,audience,status,next_index,attempts,created_at,scheduled_at FROM notification_send_tasks WHERE id=? AND created_by=?`
const HGNotificationSendCancelSQL = `UPDATE notification_send_tasks SET status='cancelled' WHERE id=? AND created_by=? AND status IN ('pending','leased','pending_approval')`

// 审批候选按 (status,id) 倒序有界读取；不读取接收人清单或变量，审核详情须再按主键读取。
const HGNotificationSendReviewPageSQL = `SELECT id,template_id,template_version,status,next_index,attempts,created_at,JSON_LENGTH(audience)
FROM notification_send_tasks WHERE status='pending_approval' AND id<? AND created_by<>? ORDER BY id DESC LIMIT ?`
const HGNotificationSendApproveLockSQL = `SELECT created_by,template_id,template_version,scheduled_at FROM notification_send_tasks WHERE id=? AND status='pending_approval' FOR UPDATE`
const HGNotificationSendApproveSQL = `UPDATE notification_send_tasks SET status='pending',approved_by=?,approved_at=UTC_TIMESTAMP(3),next_retry_at=GREATEST(COALESCE(scheduled_at,UTC_TIMESTAMP(3)),UTC_TIMESTAMP(3)) WHERE id=? AND status='pending_approval'`
const HGNotificationSendAuditSQL = `INSERT INTO notification_send_audit(task_id,action,actor_id,template_version,request_id,source_ip) VALUES (?,?,?,?,?,?)`

// 取消前锁定本人任务以在同一事务保存状态和审计；按主键精确定位。
const HGNotificationSendCancelLockSQL = `SELECT template_version,status FROM notification_send_tasks WHERE id=? AND created_by=? FOR UPDATE`

// 审核详情只包含已发布不可变版本和接收人数，不泄露用户列表或变量。
const HGNotificationSendReviewDetailSQL = `SELECT t.id,t.template_id,t.template_version,t.status,JSON_LENGTH(t.audience),t.created_at,
v.category,v.title_template,v.body_template,v.variables_schema,v.action_type,v.action_config,v.channels,t.variables,t.scheduled_at
FROM notification_send_tasks t JOIN notification_template_versions v ON v.template_id=t.template_id AND v.version=t.template_version
WHERE t.id=? AND t.status='pending_approval' AND t.created_by<>? AND v.status='published'`

// HGNotificationSendMinePageSQL 本人任务按 (created_by,id) 索引倒序游标分页；一次至多取 21 行，
// 不做跨管理员扫描或无界 COUNT；索引由 000084 迁移补齐，未迁移前不要开放该入口。
const HGNotificationSendMinePageSQL = `SELECT id,template_id,template_version,status,next_index,attempts,created_at,JSON_LENGTH(audience),scheduled_at
FROM notification_send_tasks WHERE created_by=? AND id<? ORDER BY id DESC LIMIT ?`

// HGNotificationSendPublishedPageSQL 只读取当前发布版本的快照，不能从最新草稿选择标题、变量或渠道。
// 按 000084 的 (status,id) 索引顺序游标分页，每页最多 21 行；版本表走 (template_id,version) 唯一索引。
const HGNotificationSendPublishedPageSQL = `SELECT t.id,t.template_code,v.name,v.category,t.status,v.version,t.published_version,
v.title_template,v.body_template,v.variables_schema,v.action_type,v.action_config,v.channels,v.priority,v.locale
FROM notification_templates t JOIN notification_template_versions v ON v.template_id=t.id AND v.version=t.published_version AND v.status='published'
WHERE t.status='published' AND t.id<? ORDER BY t.id DESC LIMIT ?`

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

package SQLQueriesPackage

// HGNotificationTemplatePageSQL 按主键倒序游标读取至多 21 条；只依赖主键索引，不做深 OFFSET 或全表计数。
const HGNotificationTemplatePageSQL = `SELECT t.id,t.template_code,t.name,t.category,t.status,t.latest_version,t.published_version,
v.title_template,v.body_template,v.variables_schema,v.action_type,v.action_config,v.channels,v.priority,v.locale
FROM notification_templates t JOIN notification_template_versions v ON v.template_id=t.id AND v.version=t.latest_version
WHERE t.id<? AND t.status<>'deleted' ORDER BY t.id DESC LIMIT ?`

// HGNotificationTemplateDetailSQL 按主键与版本唯一键读取一条；已删除模板不可再编辑或回填。
const HGNotificationTemplateDetailSQL = `SELECT t.id,t.template_code,t.name,t.category,t.status,t.latest_version,t.published_version,
v.title_template,v.body_template,v.variables_schema,v.action_type,v.action_config,v.channels,v.priority,v.locale
FROM notification_templates t JOIN notification_template_versions v ON v.template_id=t.id AND v.version=t.latest_version
WHERE t.id=? AND t.status<>'deleted'`

const HGNotificationTemplateInsertSQL = `INSERT INTO notification_templates(template_code,name,category,scope_type,scope_id,created_by,updated_by) VALUES (?,?,?,?,?,?,?)`
const HGNotificationTemplateVersionInsertSQL = `INSERT INTO notification_template_versions(template_id,version,name,category,title_template,body_template,variables_schema,action_type,action_config,channels,priority,locale,created_by) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`

// HGNotificationTemplateLockSQL 仅锁单个模板主键；事务中禁止外部 I/O，避免编辑与删除竞态。
const HGNotificationTemplateLockSQL = `SELECT latest_version,status,published_version FROM notification_templates WHERE id=? FOR UPDATE`
const HGNotificationTemplateUpdateSQL = `UPDATE notification_templates SET name=?,category=?,latest_version=?,updated_by=? WHERE id=?`
const HGNotificationTemplateDeleteSQL = `UPDATE notification_templates SET status='deleted',updated_by=? WHERE id=?`

// HGNotificationTemplatePublishSQL 按主键单行提交当前已锁定版本；版本内容不可变，旧发布版本仍保留。
const HGNotificationTemplatePublishSQL = `UPDATE notification_templates SET published_version=?,status='published',updated_by=? WHERE id=? AND latest_version=? AND status<>'deleted'`
const HGNotificationTemplatePublishLockSQL = `SELECT latest_version,status,template_code FROM notification_templates WHERE id=? FOR UPDATE`
const HGNotificationTemplatePublishVersionSQL = `SELECT title_template,body_template,variables_schema,channels FROM notification_template_versions WHERE template_id=? AND version=?`
const HGNotificationTemplatePublishVersionUpdateSQL = `UPDATE notification_template_versions SET status='published' WHERE template_id=? AND version=?`

// HGNotificationRolePublishedSQL 通过 template_code 唯一索引与版本唯一索引取得已发布角色通知；不得使用最新草稿发送。
const HGNotificationRolePublishedSQL = `SELECT v.title_template,v.body_template,v.variables_schema,v.channels
FROM notification_templates t JOIN notification_template_versions v ON v.template_id=t.id AND v.version=t.published_version
WHERE t.template_code='iam.role.updated' AND t.status='published' AND v.status='published'`

// HGIAMRoleTemplateStateSQL 仅按唯一编码查看角色模板是否已发布及渠道状态，不读取正文或用户资料。
const HGIAMRoleTemplateStateSQL = `SELECT t.latest_version,t.published_version,t.status,
  JSON_CONTAINS(v.channels, JSON_QUOTE('realtime'))
FROM notification_templates t JOIN notification_template_versions v ON v.template_id=t.id AND v.version=t.latest_version
WHERE t.template_code='iam.role.updated'`

// HGIAMRoleTemplateDraftSQL 只读取指定角色模板的最新草稿用于验证发布约束，不输出正文。
const HGIAMRoleTemplateDraftSQL = `SELECT v.title_template,v.body_template,v.variables_schema
FROM notification_templates t JOIN notification_template_versions v ON v.template_id=t.id AND v.version=t.latest_version
WHERE t.template_code='iam.role.updated' AND t.status<>'deleted'`

// HGIAMRoleTemplateIDSQL 仅供本机 debug 的显式验收查找已有角色模板，不会创建或修改其他模板。
const HGIAMRoleTemplateIDSQL = `SELECT id FROM notification_templates WHERE template_code='iam.role.updated'`

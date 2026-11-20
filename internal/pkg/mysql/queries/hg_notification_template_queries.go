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

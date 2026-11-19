package SQLQueriesPackage

// IAM 作用域字典以 code 唯一点查；列表按 id 游标有界读取，停用时锁定字典行并检查绑定索引。
const (
	HGSelectIAMScopeTypesFirst    = `SELECT id,scope_code,scope_name,description,requires_scope_id,status FROM iam_scope_types ORDER BY id DESC LIMIT ?`
	HGSelectIAMScopeTypesAfter    = `SELECT id,scope_code,scope_name,description,requires_scope_id,status FROM iam_scope_types WHERE id<? ORDER BY id DESC LIMIT ?`
	HGSelectIAMScopeType          = `SELECT id,scope_code,scope_name,description,requires_scope_id,status FROM iam_scope_types WHERE scope_code=? LIMIT 1`
	HGSelectIAMScopeTypeForUpdate = `SELECT status FROM iam_scope_types WHERE scope_code=? FOR UPDATE`
	HGInsertIAMScopeType          = `INSERT INTO iam_scope_types(scope_code,scope_name,description,requires_scope_id,status) VALUES(?,?,?,?,0)`
	HGUpdateIAMScopeType          = `UPDATE iam_scope_types SET scope_name=?,description=? WHERE scope_code=?`
	HGDisableIAMScopeType         = `UPDATE iam_scope_types SET status=0 WHERE scope_code=? AND status=1`
	// idx_iam_binding_scope_type 前缀点查，任何历史绑定均禁止停用。
	HGSelectIAMScopeBinding = `SELECT 1 FROM iam_user_role_bindings WHERE scope_type=? LIMIT 1`
	// 授权只接受活动且已接入权威资源校验的作用域；字典无法替代业务资源真实性验证。
	HGSelectIAMActiveScopeType = `SELECT requires_scope_id FROM iam_scope_types WHERE scope_code=? AND status=1 LIMIT 1 FOR UPDATE`
	// 运维只读诊断限定当前库的单个索引元数据；不扫描授权表数据，检查列顺序以确认迁移完整应用。
	HGSelectIAMScopeTypeIndexColumns = `SELECT COALESCE(GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX SEPARATOR ','),'') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='iam_user_role_bindings' AND INDEX_NAME='idx_iam_binding_scope_type'`
)

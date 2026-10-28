package SQLQueriesPackage

// IAM 固定 SQL 集中于此。角色、绑定均按主键/唯一键点查；列表使用 id 游标和有界 LIMIT。
const (
	// 权限点查须使用 idx_iam_binding_user_scope、role_permissions 主键及 permission_code 唯一键；调用方失败时拒绝。
	HGSelectIAMPermission = `SELECT 1 FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id AND r.status=1 JOIN iam_role_permissions rp ON rp.role_id=r.id JOIN iam_permissions p ON p.id=rp.permission_id WHERE b.user_id=? AND b.status=1 AND b.scope_type=? AND b.scope_id=? AND (b.expires_at IS NULL OR b.expires_at>CURRENT_TIMESTAMP(3)) AND p.permission_code=? LIMIT 1`
	HGSelectIAMManager    = `SELECT 1 FROM admin_user WHERE user_id=? AND status=1 AND is_delete=0 LIMIT 1`
	// 角色列表依赖 idx_iam_role_status_id，使用 id 游标，单页由 Service 限制到 100。
	HGSelectIAMRolesFirst             = `SELECT id,role_code,role_name,description,status,built_in,created_at FROM iam_roles WHERE status=1 ORDER BY id DESC LIMIT ?`
	HGSelectIAMRolesAfter             = `SELECT id,role_code,role_name,description,status,built_in,created_at FROM iam_roles WHERE status=1 AND id<? ORDER BY id DESC LIMIT ?`
	HGSelectIAMRole                   = `SELECT id,role_code,role_name,description,status,built_in,created_at FROM iam_roles WHERE id=? LIMIT 1`
	HGSelectIAMActiveRoleCode         = `SELECT role_code FROM iam_roles WHERE id=? AND status=1 LIMIT 1 FOR UPDATE`
	HGSelectIAMBindingRoleForRevoke   = `SELECT r.role_code FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id WHERE b.id=? AND b.user_id=? AND b.status=1 LIMIT 1 FOR UPDATE`
	HGSelectIAMRoleForPermissionWrite = `SELECT built_in FROM iam_roles WHERE id=? AND status=1 LIMIT 1 FOR UPDATE`
	HGInsertIAMRole                   = `INSERT INTO iam_roles(role_code,role_name,description,status,built_in) VALUES(?,?,?,1,0)`
	HGUpdateIAMRole                   = `UPDATE iam_roles SET role_name=?,description=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status=1 AND built_in=0`
	HGDisableIAMRole                  = `UPDATE iam_roles SET status=0,updated_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status=1 AND built_in=0 AND NOT EXISTS(SELECT 1 FROM iam_user_role_bindings WHERE role_id=? AND status=1 AND (expires_at IS NULL OR expires_at>CURRENT_TIMESTAMP(3)) LIMIT 1)`
	// 员工绑定列表依赖 idx_iam_binding_user_page，精确 user_id 加 id 游标；已过期或停用角色不展示。
	HGSelectIAMBindingsFirst = `SELECT b.id,b.user_id,b.role_id,r.role_code,b.scope_type,b.scope_id,b.expires_at FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id AND r.status=1 WHERE b.user_id=? AND b.status=1 AND (b.expires_at IS NULL OR b.expires_at>CURRENT_TIMESTAMP(3)) ORDER BY b.id DESC LIMIT ?`
	HGSelectIAMBindingsAfter = `SELECT b.id,b.user_id,b.role_id,r.role_code,b.scope_type,b.scope_id,b.expires_at FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id AND r.status=1 WHERE b.user_id=? AND b.status=1 AND b.id<? AND (b.expires_at IS NULL OR b.expires_at>CURRENT_TIMESTAMP(3)) ORDER BY b.id DESC LIMIT ?`
	HGSelectIAMBinding       = `SELECT id,CASE WHEN expires_at IS NOT NULL AND expires_at<=CURRENT_TIMESTAMP(3) THEN 0 ELSE status END FROM iam_user_role_bindings WHERE user_id=? AND role_id=? AND scope_type=? AND scope_id=? FOR UPDATE`
	HGInsertIAMBinding       = `INSERT INTO iam_user_role_bindings(user_id,role_id,scope_type,scope_id,status,expires_at,granted_by) VALUES(?,?,?,?,1,?,?)`
	HGReactivateIAMBinding   = `UPDATE iam_user_role_bindings SET status=1,expires_at=?,granted_by=?,revoked_at=NULL,updated_at=CURRENT_TIMESTAMP(3) WHERE id=?`
	HGRevokeIAMBinding       = `UPDATE iam_user_role_bindings SET status=0,revoked_at=CURRENT_TIMESTAMP(3),updated_at=CURRENT_TIMESTAMP(3) WHERE id=? AND user_id=? AND status=1`
	// 审计与授权变更同事务插入；审计表按 actor/target 时间索引查询，归档由独立任务负责。
	HGInsertIAMAudit           = `INSERT INTO iam_audit_logs(actor_user_id,action,target_type,target_id,request_id,detail) VALUES(?,?,?,?,?,?)`
	HGSelectIAMPermissions     = `SELECT permission_code,description FROM iam_permissions ORDER BY permission_code LIMIT ?`
	HGSelectIAMRolePermissions = `SELECT p.permission_code FROM iam_role_permissions rp JOIN iam_permissions p ON p.id=rp.permission_id WHERE rp.role_id=? ORDER BY p.permission_code LIMIT ?`
	HGDeleteIAMRolePermissions = `DELETE FROM iam_role_permissions WHERE role_id=?`
	HGBumpIAMRoleVersion       = `UPDATE iam_roles SET version=version+1,updated_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status=1 AND built_in=0`
	HGInsertIAMRolePermission  = `INSERT INTO iam_role_permissions(role_id,permission_id) SELECT ?,id FROM iam_permissions WHERE permission_code=? LIMIT 1`
	HGSelectIAMOrganization    = `SELECT 1 FROM iam_organizations WHERE id=? AND org_type=? AND status=1 LIMIT 1`
)

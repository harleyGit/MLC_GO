package SQLQueriesPackage

// 仅供本机 debug 单次引导：主键/唯一键点查，受控事务写入；执行完成后移除。
const (
	HGSelectLocalMediaBootstrapRole            = `SELECT id FROM iam_roles WHERE role_code=? LIMIT 1`
	HGSelectLocalMediaBootstrapPermission      = `SELECT id FROM iam_permissions WHERE permission_code=? LIMIT 1`
	HGSelectLocalMediaBootstrapDatabase        = `SELECT DATABASE()`
	HGSelectLocalMediaBootstrapAudit           = `SELECT COUNT(*) FROM iam_audit_logs WHERE request_id=? AND action IN ('iam.role.create','iam.binding.grant')`
	HGSelectLocalMediaBootstrapRolePermissions = `SELECT COUNT(*) FROM iam_role_permissions WHERE role_id=?`
	HGSelectLocalMediaBootstrapRoleBindings    = `SELECT COUNT(*) FROM iam_user_role_bindings WHERE role_id=?`
)

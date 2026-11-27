package hgerrors

const (
	HGOPSOperationForbidden     Code = 1801001
	HGOPSApprovalRequired       Code = 1801002
	HGOPSMFARequired            Code = 1801003
	HGOPSConfigVersionConflict  Code = 1802001
	HGOPSConfigValidationFailed Code = 1802002
	HGOPSServiceUnhealthy       Code = 1803001
	HGOPSDeploymentInProgress   Code = 1803002
	HGOPSAuditWriteFailed       Code = 1804001
	HGOPSProtectedResource      Code = 1804002
)

var hgOps = []Definition{
	{HGOPSOperationForbidden, "OPS_OPERATION_FORBIDDEN", "运维操作被拒绝", 403, false},
	{HGOPSApprovalRequired, "OPS_APPROVAL_REQUIRED", "需要审批", 403, false},
	{HGOPSMFARequired, "OPS_MFA_REQUIRED", "需要 MFA 验证", 403, false},
	{HGOPSConfigVersionConflict, "OPS_CONFIG_VERSION_CONFLICT", "配置版本冲突", 409, false},
	{HGOPSConfigValidationFailed, "OPS_CONFIG_VALIDATION_FAILED", "配置校验失败", 400, false},
	{HGOPSServiceUnhealthy, "OPS_SERVICE_UNHEALTHY", "服务健康检查失败", 503, true},
	{HGOPSDeploymentInProgress, "OPS_DEPLOYMENT_IN_PROGRESS", "正在部署中", 409, false},
	{HGOPSAuditWriteFailed, "OPS_AUDIT_WRITE_FAILED", "审计日志持久化失败", 503, true},
	{HGOPSProtectedResource, "OPS_PROTECTED_RESOURCE", "受保护资源不可删除", 403, false},
}

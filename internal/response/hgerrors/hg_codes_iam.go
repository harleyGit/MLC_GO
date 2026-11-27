package hgerrors

const (
	HGIAMRoleNotFound            Code = 1201001
	HGIAMRoleNotAssigned         Code = 1201002
	HGIAMPermissionDenied        Code = 1201003
	HGIAMScopeDenied             Code = 1201004
	HGIAMRoleBindingConflict     Code = 1201005
	HGIAMGrantTargetInvalid      Code = 1201006
	HGIAMPolicyVersionStale      Code = 1202001
	HGIAMHighRiskConfirmRequired Code = 1202002
	HGIAMProtectedRole           Code = 1202003
)

var hgIam = []Definition{
	{HGIAMRoleNotFound, "IAM_ROLE_NOT_FOUND", "角色不存在", 404, false},
	{HGIAMRoleNotAssigned, "IAM_ROLE_NOT_ASSIGNED", "未分配所需角色", 403, false},
	{HGIAMPermissionDenied, "IAM_PERMISSION_DENIED", "没有操作权限", 403, false},
	{HGIAMScopeDenied, "IAM_SCOPE_DENIED", "不在授权作用域内", 403, false},
	{HGIAMRoleBindingConflict, "IAM_ROLE_BINDING_CONFLICT", "角色绑定冲突", 409, false},
	{HGIAMGrantTargetInvalid, "IAM_GRANT_TARGET_INVALID", "目标不可授权", 404, false},
	{HGIAMPolicyVersionStale, "IAM_POLICY_VERSION_STALE", "权限策略版本过期", 409, false},
	{HGIAMHighRiskConfirmRequired, "IAM_HIGH_RISK_CONFIRM_REQUIRED", "需要高风险操作确认", 403, false},
	{HGIAMProtectedRole, "IAM_PROTECTED_ROLE", "禁止修改受保护角色", 403, false},
}

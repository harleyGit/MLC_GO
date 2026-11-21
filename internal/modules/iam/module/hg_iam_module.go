package module

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	IAMPackage "MLC_GO/internal/modules/iam"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"net/http"
)

const hgBasePath = "/api/v1/iam"

// HGModule 将 IAM 接口注册到根路由，复用 JWT 和 API Guard。
type HGModule struct {
	handler      *IAMPackage.HGHandler             // 模块私有 HTTP 处理器。
	hgAuthorizer HGRouterPackage.HGRouteAuthorizer // IAM 活动管理员及全局权限校验。
}

// RegisterHGModule 注入共享 SQL 连接并注册 IAM 模块。
func RegisterHGModule(sqlManager *PersistenceSQLPackage.HGSQLManager, caches ...*PersistenceRedisPackage.RedisService) *IAMPackage.HGService {
	repo := IAMPackage.NewHGRepository(sqlManager.GetSQLDB())
	if len(caches) > 0 {
		repo.WithHGPermissionCache(caches[0])
	}
	hgService := IAMPackage.NewHGService(repo)
	HGHandlerPackage.RegisterModule(&HGModule{handler: IAMPackage.NewHGHandler(hgService), hgAuthorizer: IAMPackage.NewHGAssetAuthorizer(hgService)})
	return hgService
}

// Name 返回模块注册名。
func (m *HGModule) Name() string { return "iam" }

// BasePath 返回 IAM API 前缀。
func (m *HGModule) BasePath() string { return hgBasePath }

// Handler 返回带 JWT 和 API Guard 的路由组。
func (m *HGModule) Handler() http.Handler {
	return HGRouterPackage.NewRouteGroup(HGRouterPackage.RouteGroupConfig{BasePath: hgBasePath, Authorizer: m.hgAuthorizer, AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware}, hgRoutes(m.handler))
}

// RouteCatalog 返回 IAM 路由清单。
func RouteCatalog() []HGRouterPackage.RouteCatalogItem {
	return HGRouterPackage.BuildRouteCatalogItems(hgRoutes(nil))
}

func hgRoutes(handler *IAMPackage.HGHandler) []HGRouterPackage.RouteSpec {
	var meAccess, authorizeAccess, realtimeToken http.HandlerFunc
	var list, create, update, disable, bindings, pendingGrants, grant, directGrant, revoke, permissions, setPermissions, mfaEnroll, mfaConfirm, approvals, approve, reject http.HandlerFunc
	var scopeList, scopeGet, scopeCreate, scopeUpdate, scopeDisable, batchGrant http.HandlerFunc
	if handler != nil {
		meAccess, authorizeAccess = handler.HGMeAccess, handler.HGAuthorizeAccess
		realtimeToken = handler.HGRealtimeToken
		batchGrant = handler.HGGrantDirectBatch
		scopeList, scopeGet, scopeCreate, scopeUpdate, scopeDisable = handler.HGListScopeTypes, handler.HGGetScopeType, handler.HGCreateScopeType, handler.HGUpdateScopeType, handler.HGDisableScopeType
		list, create, update, disable = handler.HGListRoles, handler.HGCreateRole, handler.HGUpdateRole, handler.HGDisableRole
		bindings, pendingGrants, grant, revoke = handler.HGListBindings, handler.HGListPendingGrants, handler.HGGrant, handler.HGRevoke
		directGrant = handler.HGGrantDirect
		permissions, setPermissions = handler.HGListPermissions, handler.HGSetRolePermissions
		mfaEnroll, mfaConfirm = handler.HGEnrollMFA, handler.HGConfirmMFA
		approvals, approve, reject = handler.HGListApprovals, handler.HGApproveApproval, handler.HGRejectApproval
	}
	// 管理端权限是带字符串值的 HGPermission 枚举：HGIAMRoleRead="iam:role:read"，
	// HGIAMRoleGrant="iam:role:grant"，HGIAMRoleGrantDirect="iam:role:grant_direct"，HGIAMApprovalApprove="iam:approval:approve"。
	// 除 JWT 外，授权器还要求当前 user_id 为活动管理员，且有未过期的全局角色绑定及对应权限。
	return []HGRouterPackage.RouteSpec{
		// 本人权限快照与全局权限预判仅要求 JWT；具体业务资源仍由业务 API 独立鉴权。
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/me/access", true, "获取本人授权快照", meAccess),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/me/sync", true, "实时重连同步本人授权快照", meAccess),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/me/realtime", true, "签发本人 IAM 实时频道凭据", realtimeToken),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/authorize", true, "全局权限预判", authorizeAccess),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/scope-types/list", true, "查询作用域类型", scopeList, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/scope-types/detail", true, "查询作用域类型详情", scopeGet, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/scope-types", true, "创建作用域类型", scopeCreate, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/scope-types/update", true, "更新作用域类型", scopeUpdate, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/scope-types/disable", true, "停用作用域类型", scopeDisable, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/roles/list", true, "查询角色", list, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/roles", true, "创建角色", create, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/roles/update", true, "更新角色", update, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/roles/disable", true, "停用角色", disable, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/bindings/list", true, "查询员工角色", bindings, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/bindings/pending", true, "查询目标员工待审批角色", pendingGrants, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/bindings/grant", true, "授予员工角色", grant, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/bindings/grant-direct", true, "直接授予员工角色", directGrant, HGPermissionPackage.HGIAMRoleGrant, HGPermissionPackage.HGIAMRoleGrantDirect),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/bindings/grant-direct-batch", true, "批量直接授予员工角色", batchGrant, HGPermissionPackage.HGIAMRoleGrant, HGPermissionPackage.HGIAMRoleGrantDirect),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/bindings/revoke", true, "撤销员工角色", revoke, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/permissions/list", true, "查询权限目录", permissions, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/roles/permissions", true, "设置角色权限", setPermissions, HGPermissionPackage.HGIAMRoleGrant),
		// /mfa/enroll 仅为当前管理员绑定自己的 TOTP，不授予 IAM 权限：还要复核本人登录密码、检查 Redis 尝试次数和 MFA 加密密钥。
		// /mfa/confirm 使用绑定时返回的密钥生成的六位动态码启用因子；两者均要求 HGIAMRoleGrant，不接受代他人绑定。
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/mfa/enroll", true, "绑定 TOTP 因子", mfaEnroll, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/mfa/confirm", true, "启用 TOTP 因子", mfaConfirm, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/approvals/list", true, "查询待审批授权", approvals, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/approvals/approve", true, "批准授权", approve, HGPermissionPackage.HGIAMRoleGrant, HGPermissionPackage.HGIAMApprovalApprove),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/approvals/reject", true, "拒绝授权", reject, HGPermissionPackage.HGIAMRoleGrant, HGPermissionPackage.HGIAMApprovalApprove),
	}
}

package module

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	IAMPackage "MLC_GO/internal/modules/iam"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	HGServerPackage "MLC_GO/internal/pkg/server"
	"net/http"
)

const hgBasePath = "/api/v1/iam"

// HGModule 将 IAM 接口注册到根路由，复用 JWT 和 API Guard。
type HGModule struct {
	handler      *IAMPackage.HGHandler             // 模块私有 HTTP 处理器。
	hgAuthorizer HGRouterPackage.HGRouteAuthorizer // IAM 活动管理员及全局权限校验。
}

// RegisterHGModule 注入共享 SQL 连接并注册 IAM 模块。
func RegisterHGModule(sqlManager *PersistenceSQLPackage.HGSQLManager, caches ...*PersistenceRedisPackage.RedisService) {
	repo := IAMPackage.NewHGRepository(sqlManager.GetSQLDB())
	if len(caches) > 0 {
		repo.WithHGPermissionCache(caches[0])
	}
	hgService := IAMPackage.NewHGService(repo)
	HGHandlerPackage.RegisterModule(&HGModule{handler: IAMPackage.NewHGHandler(hgService), hgAuthorizer: IAMPackage.NewHGAssetAuthorizer(hgService)})
}

// Name 返回模块注册名。
func (m *HGModule) Name() string { return "iam" }

// BasePath 返回 IAM API 前缀。
func (m *HGModule) BasePath() string { return hgBasePath }

// Handler 返回带 JWT 和 API Guard 的路由组。
func (m *HGModule) Handler() http.Handler {
	return HGRouterPackage.NewRouteGroup(HGRouterPackage.RouteGroupConfig{BasePath: hgBasePath, Rules: HGServerPackage.IAMMethodRules(), Authorizer: m.hgAuthorizer, AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware}, hgRoutes(m.handler))
}

// RouteCatalog 返回 IAM 路由清单。
func RouteCatalog() []HGRouterPackage.RouteCatalogItem {
	return HGRouterPackage.BuildRouteCatalogItems(hgRoutes(nil))
}

func hgRoutes(handler *IAMPackage.HGHandler) []HGRouterPackage.RouteSpec {
	var list, create, update, disable, bindings, grant, revoke, permissions, setPermissions, mfaEnroll, mfaConfirm, approvals, approve, reject http.HandlerFunc
	if handler != nil {
		list, create, update, disable = handler.HGListRoles, handler.HGCreateRole, handler.HGUpdateRole, handler.HGDisableRole
		bindings, grant, revoke = handler.HGListBindings, handler.HGGrant, handler.HGRevoke
		permissions, setPermissions = handler.HGListPermissions, handler.HGSetRolePermissions
		mfaEnroll, mfaConfirm = handler.HGEnrollMFA, handler.HGConfirmMFA
		approvals, approve, reject = handler.HGListApprovals, handler.HGApproveApproval, handler.HGRejectApproval
	}
	return []HGRouterPackage.RouteSpec{
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/roles/list", true, "查询角色", list, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/roles", true, "创建角色", create, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/roles/update", true, "更新角色", update, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/roles/disable", true, "停用角色", disable, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/bindings/list", true, "查询员工角色", bindings, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/bindings/grant", true, "授予员工角色", grant, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/bindings/revoke", true, "撤销员工角色", revoke, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/permissions/list", true, "查询权限目录", permissions, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/roles/permissions", true, "设置角色权限", setPermissions, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/mfa/enroll", true, "绑定 TOTP 因子", mfaEnroll, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/mfa/confirm", true, "启用 TOTP 因子", mfaConfirm, HGPermissionPackage.HGIAMRoleGrant),
		HGRouterPackage.NewRouteSpec("iam", http.MethodGet, hgBasePath, "/approvals/list", true, "查询待审批授权", approvals, HGPermissionPackage.HGIAMRoleRead),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/approvals/approve", true, "批准授权", approve, HGPermissionPackage.HGIAMRoleGrant, HGPermissionPackage.HGIAMApprovalApprove),
		HGRouterPackage.NewRouteSpec("iam", http.MethodPost, hgBasePath, "/approvals/reject", true, "拒绝授权", reject, HGPermissionPackage.HGIAMRoleGrant, HGPermissionPackage.HGIAMApprovalApprove),
	}
}

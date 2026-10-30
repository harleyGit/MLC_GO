package module

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	IAMPackage "MLC_GO/internal/modules/iam"
	NotificationPackage "MLC_GO/internal/modules/notification"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	"net/http"
)

// HGModule 将站内通知、偏好、受权限约束的管理视图分别挂载；不承载 IM 私信。
type HGModule struct {
	base       string                            // 路由前缀，仅限下述三类协议入口。
	handler    *NotificationPackage.HGHandler    // 复用同一通知业务实例。
	authorizer HGRouterPackage.HGRouteAuthorizer // 管理入口 IAM 校验器。
}

func (m *HGModule) Name() string     { return "notification" }
func (m *HGModule) BasePath() string { return m.base }
func (m *HGModule) Handler() http.Handler {
	return HGRouterPackage.NewRouteGroup(HGRouterPackage.RouteGroupConfig{BasePath: m.base, AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware, Authorizer: m.authorizer}, hgRoutes(m.base, m.handler))
}

// HGRegisterModules 注入共享连接池与 IAM；生产者可通过返回的 Service 调用可信站内投递。
func HGRegisterModules(sqlManager *PersistenceSQLPackage.HGSQLManager, authorizer HGRouterPackage.HGRouteAuthorizer) *NotificationPackage.HGService {
	service := NotificationPackage.HGNewService(NotificationPackage.HGNewRepository(sqlManager.GetSQLDB()))
	handler := NotificationPackage.HGNewHandler(service)
	for _, base := range []string{"/api/v1/notifications", "/api/v1/notification", "/admin/v1/notification"} {
		HGHandlerPackage.RegisterModule(&HGModule{base: base, handler: handler, authorizer: authorizer})
	}
	return service
}

// HGNewAuthorizer 使用现有 IAM 权限缓存与同一数据库，不从前端角色标记推断管理员权限。
func HGNewAuthorizer(sqlManager *PersistenceSQLPackage.HGSQLManager) HGRouterPackage.HGRouteAuthorizer {
	return IAMPackage.NewHGAssetAuthorizer(IAMPackage.NewHGService(IAMPackage.NewHGRepository(sqlManager.GetSQLDB())))
}

// HGRouteCatalog 列出当前真正可用的通知接口，不将未实现的 IM 路由注册为成功接口。
func HGRouteCatalog() []HGRouterPackage.RouteCatalogItem {
	items := []HGRouterPackage.RouteCatalogItem{}
	for _, base := range []string{"/api/v1/notifications", "/api/v1/notification", "/admin/v1/notification"} {
		items = append(items, HGRouterPackage.BuildRouteCatalogItems(hgRoutes(base, nil))...)
	}
	return items
}

func hgRoutes(base string, h *NotificationPackage.HGHandler) []HGRouterPackage.RouteSpec {
	var list, unread, read, del, getPref, setPref, channels, deliveries, testSend http.HandlerFunc
	if h != nil {
		list, unread, read, del = h.HGList, h.HGUnread, h.HGRead, h.HGDelete
		getPref, setPref = h.HGGetPreferences, h.HGSetPreferences
		channels, deliveries = h.HGChannels, h.HGDeliveries
		testSend = h.HGTestSend
	}
	switch base {
	case "/api/v1/notifications":
		return []HGRouterPackage.RouteSpec{
			HGRouterPackage.NewRouteSpec("notification", http.MethodGet, base, "/", true, "本人站内通知游标列表", list),
			HGRouterPackage.NewRouteSpec("notification", http.MethodGet, base, "/unread", true, "本人通知未读汇总", unread),
			HGRouterPackage.NewRouteSpec("notification", http.MethodPost, base, "/read", true, "本人通知有界已读", read),
			HGRouterPackage.NewRouteSpec("notification", http.MethodPost, base, "/delete", true, "本人通知软删除", del),
		}
	case "/api/v1/notification":
		return []HGRouterPackage.RouteSpec{
			HGRouterPackage.NewRouteSpec("notification", http.MethodGet, base, "/preferences", true, "本人通知偏好", getPref),
			HGRouterPackage.NewRouteSpec("notification", http.MethodPut, base, "/preferences", true, "修改本人通知偏好", setPref),
		}
	default:
		return []HGRouterPackage.RouteSpec{
			HGRouterPackage.NewRouteSpec("notification", http.MethodGet, base, "/channels", true, "可用通知渠道", channels, HGPermissionPackage.HGNotificationChannelRead),
			HGRouterPackage.NewRouteSpec("notification", http.MethodGet, base, "/deliveries", true, "站内投递记录游标列表", deliveries, HGPermissionPackage.HGNotificationDeliveryRead),
			HGRouterPackage.NewRouteSpec("notification", http.MethodPost, base, "/test-send", true, "向管理员本人发送站内试通知", testSend, HGPermissionPackage.HGNotificationChannelSend),
		}
	}
}

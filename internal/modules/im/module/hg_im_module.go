package module

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	IMPackage "MLC_GO/internal/modules/im"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	"net/http"
)

// HGModule 仅提供 JWT 鉴权的单聊 HTTP 权威接口，实时网关与媒体授权尚未对外开放。
type HGModule struct {
	handler *IMPackage.HGHandler // 与业务路由共享同一 IM 服务实例。
}

func (m *HGModule) Name() string     { return "im" }
func (m *HGModule) BasePath() string { return "/api/v1/im" }
func (m *HGModule) Handler() http.Handler {
	return HGRouterPackage.NewRouteGroup(HGRouterPackage.RouteGroupConfig{BasePath: m.BasePath(), AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware}, hgRoutes(m.handler))
}

// HGRegisterModules 通过现有领域事件 topic 持久化已提交 IM 事件；不得把 HTTP ACK 解释为实时送达。
func HGRegisterModules(sqlManager *PersistenceSQLPackage.HGSQLManager, topic string) {
	service := IMPackage.HGNewService(sqlManager.GetSQLDB(), topic, nil)
	HGHandlerPackage.RegisterModule(&HGModule{handler: IMPackage.HGNewHandler(service)})
}

func HGRouteCatalog() []HGRouterPackage.RouteCatalogItem {
	return HGRouterPackage.BuildRouteCatalogItems(hgRoutes(nil))
}

func hgRoutes(handler *IMPackage.HGHandler) []HGRouterPackage.RouteSpec {
	var list, create, messages, send, read, registerDevice, pendingDevice, ackDevice http.HandlerFunc
	if handler != nil {
		list, create, messages, send, read = handler.HGConversations, handler.HGCreateConversation, handler.HGMessages, handler.HGSend, handler.HGRead
		registerDevice, pendingDevice, ackDevice = handler.HGRegisterDevice, handler.HGDevicePending, handler.HGAckDevice
	}
	base := "/api/v1/im"
	return []HGRouterPackage.RouteSpec{
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/conversations", true, "本人单聊会话游标列表", list),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/conversations", true, "创建本人单聊会话", create),
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/messages", true, "本人会话 seq 补拉", messages),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/messages", true, "发送单聊文本消息", send),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/read", true, "上报本人会话已读水位", read),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/device/register", true, "注册本人随机设备", registerDevice),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/device/pending", true, "补拉本设备待确认消息指针", pendingDevice),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/device/ack", true, "客户端本设备确认消息", ackDevice),
	}
}

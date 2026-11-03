package module

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	IMPackage "MLC_GO/internal/modules/im"
	Contacts "MLC_GO/internal/modules/im_contacts"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"database/sql"
	"net/http"
)

// HGModule 提供 JWT 鉴权的 IM 权威接口；外部能力按显式配置启用，缺配置时失败关闭。
type HGModule struct {
	handler  *IMPackage.HGHandler // 与业务路由共享同一 IM 服务实例。
	contacts *Contacts.HGHandler  // 联系人关系独立领域，复用现有 JWT 和路由保护。
}

func (m *HGModule) Name() string     { return "im" }
func (m *HGModule) BasePath() string { return "/api/v1/im" }
func (m *HGModule) Handler() http.Handler {
	return HGRouterPackage.NewRouteGroup(HGRouterPackage.RouteGroupConfig{BasePath: m.BasePath(), AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware}, hgRoutes(m.handler, m.contacts))
}

// HGRegisterModules 通过现有领域事件 topic 持久化已提交 IM 事件；不得把 HTTP ACK 解释为实时送达。
func HGRegisterModules(sqlManager *PersistenceSQLPackage.HGSQLManager, topic string) {
	service := IMPackage.HGNewService(sqlManager.GetSQLDB(), topic, nil).HGWithSessionRedis(PersistenceRedisPackage.RDB)
	HGHandlerPackage.RegisterModule(&HGModule{handler: IMPackage.HGNewHandler(service), contacts: Contacts.HGNewHandler(sqlManager.GetSQLDB())})
}

func hgRegisterHandler(handler *IMPackage.HGHandler, db *sql.DB) {
	HGHandlerPackage.RegisterModule(&HGModule{handler: handler, contacts: Contacts.HGNewHandler(db)})
}

func HGRouteCatalog() []HGRouterPackage.RouteCatalogItem {
	return HGRouterPackage.BuildRouteCatalogItems(hgRoutes(nil, nil))
}

func hgRoutes(handler *IMPackage.HGHandler, contacts *Contacts.HGHandler) []HGRouterPackage.RouteSpec {
	var list, create, messages, send, read, registerDevice, pendingDevice, ackDevice http.HandlerFunc
	var realtime, uploadInit, uploadComplete, mediaStatus, mediaRead, callCreate, callGet, callChange, callToken http.HandlerFunc
	var revokeDevice, deleteMedia http.HandlerFunc
	var friends, incoming, blocks, profile, search, request, accept, reject, edit, remove, block, unblock http.HandlerFunc
	if handler != nil {
		list, create, messages, send, read = handler.HGConversations, handler.HGCreateConversation, handler.HGMessages, handler.HGSend, handler.HGRead
		registerDevice, pendingDevice, ackDevice = handler.HGRegisterDevice, handler.HGDevicePending, handler.HGAckDevice
		realtime, uploadInit, uploadComplete, mediaStatus, mediaRead = handler.HGRealtimeToken, handler.HGUploadInit, handler.HGUploadComplete, handler.HGMediaStatus, handler.HGMediaRead
		callCreate, callGet, callChange, callToken = handler.HGCallCreate, handler.HGCallGet, handler.HGCallChange, handler.HGCallToken
		revokeDevice, deleteMedia = handler.HGDeviceRevoke, handler.HGMediaDelete
	}
	if contacts != nil {
		friends, incoming, blocks, profile, search, request = contacts.HGFriends, contacts.HGIncoming, contacts.HGBlocks, contacts.HGProfile, contacts.HGSearch, contacts.HGSend
		accept, reject, edit = contacts.HGDecision(true), contacts.HGDecision(false), contacts.HGEdit
		remove, block, unblock = contacts.HGAction("remove"), contacts.HGAction("block"), contacts.HGAction("unblock")
	}
	base := "/api/v1/im"
	return []HGRouterPackage.RouteSpec{
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/contacts/friends", true, "本人好友游标列表", friends),
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/contacts/requests/incoming", true, "本人收到的申请游标列表", incoming),
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/contacts/blocks", true, "本人有方向黑名单游标列表", blocks),
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/contacts/profile", true, "用户公开资料，不返回隐私字段", profile),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/contacts/search", true, "按公开 ID、目标用户 ID、手机号、邮箱或自定义 ID 精确搜索", search),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/contacts/requests", true, "发起好友申请", request),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/contacts/requests/accept", true, "接受本人收到的申请", accept),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/contacts/requests/reject", true, "拒绝本人收到的申请", reject),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/contacts/friends/edit", true, "编辑本人单向备注", edit),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/contacts/friends/remove", true, "删除双方好友关系但保留历史", remove),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/contacts/blocks/add", true, "本人拉黑对方并解除好友关系", block),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/contacts/blocks/remove", true, "解除本人发起的拉黑", unblock),
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/conversations", true, "本人单聊会话游标列表", list),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/conversations", true, "创建本人单聊会话", create),
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/messages", true, "本人会话 seq 补拉", messages),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/messages", true, "发送单聊文本消息", send),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/read", true, "上报本人会话已读水位", read),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/device/register", true, "注册本人随机设备", registerDevice),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/device/pending", true, "补拉本设备待确认消息指针", pendingDevice),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/device/ack", true, "客户端本设备确认消息", ackDevice),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/device/revoke", true, "撤销本人设备凭据", revokeDevice),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/realtime/token", true, "本设备 Centrifugo 连接凭证；需启用实时能力", realtime),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/media/upload/init", true, "申请会话绑定私有上传；需启用媒体能力", uploadInit),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/media/upload/complete", true, "申请后台审核；不表示审核通过", uploadComplete),
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/media/status", true, "本人上传审核状态", mediaStatus),
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/media/read", true, "逐请求鉴权的私有媒体流", mediaRead),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/media/delete", true, "撤销本人私有媒体读取与发送", deleteMedia),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/calls", true, "发起单聊呼叫；需启用通话能力", callCreate),
		HGRouterPackage.NewRouteSpec("im", http.MethodGet, base, "/calls", true, "本人通话状态", callGet),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/calls/action", true, "接听、拒绝、取消、结束通话", callChange),
		HGRouterPackage.NewRouteSpec("im", http.MethodPost, base, "/calls/token", true, "指定房间和设备的 LiveKit 凭证", callToken),
	}
}

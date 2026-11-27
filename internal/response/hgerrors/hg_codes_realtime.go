package hgerrors

const (
	HGRealtimeTokenExpired      Code = 2101001
	HGRealtimeChannelForbidden  Code = 2101002
	HGRealtimeConnectionLimited Code = 2101003
	HGRealtimeSyncRequired      Code = 2102001
)

var hgRealtime = []Definition{
	{HGRealtimeTokenExpired, "REALTIME_TOKEN_EXPIRED", "实时连接 Token 过期", 401, false},
	{HGRealtimeChannelForbidden, "REALTIME_CHANNEL_FORBIDDEN", "无权订阅频道", 403, false},
	{HGRealtimeConnectionLimited, "REALTIME_CONNECTION_LIMITED", "连接数量超限", 429, true},
	{HGRealtimeSyncRequired, "REALTIME_SYNC_REQUIRED", "需要补齐断线消息", 409, false},
}

package iam

// HGRealtimeGrant 使用独立 IM 网关连接密钥签发本人私有频道；服务端不接受客户端指定频道。
type HGRealtimeGrant interface {
	HGUserToken(string) (string, error)
}

// HGWithRealtime 必须在 HTTP 服务器启动前注入，不在请求期间改写配置。
func (s *HGService) HGWithRealtime(gateway HGRealtimeGrant, url string) {
	s.realtime = gateway
	s.realtimeURL = url
}

package module

import "testing"

func TestHGIMMessagesRequireSocket(t *testing.T) {
	for _, route := range hgRoutes(nil, nil) {
		switch route.FullPath {
		case "/api/v1/im/messages", "/api/v1/im/device/pending", "/api/v1/im/device/ack", "/api/v1/im/media/read":
			t.Fatalf("消息/私有读取旧 HTTP 路由不得再注册: %s", route.FullPath)
		}
	}
}

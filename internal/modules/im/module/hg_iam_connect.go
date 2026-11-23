package module

import (
	"MLC_GO/internal/modules/im/transport"
	UserService "MLC_GO/internal/modules/user/service"
	Redis "MLC_GO/internal/pkg/redis"
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// hgIAMConnectHandler 是网关的私有 connect proxy；无登录会话或 Redis 不可用时拒绝重连。
// 此入口必须仅对可信网关可达，且静态共享密钥不能由客户端转发头覆盖。
func hgIAMConnectHandler(gateway *transport.HGCentrifugo, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || len(secret) < 32 ||
			subtle.ConstantTimeCompare([]byte(r.Header.Get("X-IM-RPC-Secret")), []byte(secret)) != 1 {
			http.Error(w, "网关请求无效", http.StatusForbidden)
			return
		}
		var request struct {
			Data struct {
				Token string `json:"token"` // 网关连接授权票据；只用于服务器间验证，不打印。
			} `json:"data"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if json.NewDecoder(r.Body).Decode(&request) != nil || len(request.Data.Token) > 3000 {
			http.Error(w, "连接请求不合法", http.StatusBadRequest)
			return
		}
		user, session, err := gateway.HGVerifyIAMGrant(request.Data.Token)
		if err != nil || Redis.RDB == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"disconnect": map[string]any{"code": 4501, "reason": "unauthorized"}})
			return
		}
		deleted, deleteErr := UserService.HGUserDeleted(r.Context(), Redis.RDB, user)
		if deleteErr != nil || deleted {
			_ = json.NewEncoder(w).Encode(map[string]any{"disconnect": map[string]any{"code": 4501, "reason": "unauthorized"}})
			return
		}
		active, sessionErr := UserService.HGSessionActive(r.Context(), Redis.RDB, user, session)
		if sessionErr != nil || !active {
			_ = json.NewEncoder(w).Encode(map[string]any{"disconnect": map[string]any{"code": 4501, "reason": "unauthorized"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
			"user": "iam/" + user + "/" + session, "channels": []string{transport.HGBusinessChannel(user)},
		}})
	})
}

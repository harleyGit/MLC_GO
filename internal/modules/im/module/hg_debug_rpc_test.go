package module

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHGMountDebugRPCOnlyInDebug(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want int
	}{
		{env: "debug", want: http.StatusNoContent},
		{env: "pre", want: http.StatusNotFound},
		{env: "prod", want: http.StatusNotFound},
	} {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv("SERVER_ENV", tc.env)
			mux := http.NewServeMux()
			(&HGRuntime{rpc: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})}).HGMountDebugRPC(mux)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/internal/im/centrifugo/rpc", nil))
			if w.Code != tc.want {
				t.Fatalf("%s 下 RPC 路由状态 = %d，期望 %d", tc.env, w.Code, tc.want)
			}
		})
	}
}

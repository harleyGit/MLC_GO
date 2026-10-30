package OpsHandlerPackage

import (
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type hgSpiderSwitchStoreStub struct {
	called bool // 只有获得权限后才允许触及数据库。
}

func (s *hgSpiderSwitchStoreStub) HGDesiredRunning(context.Context) (bool, error) { return false, nil }
func (s *hgSpiderSwitchStoreStub) HGSetDesiredRunning(context.Context, bool, string) (bool, error) {
	s.called = true
	return true, nil
}

type hgSpiderSwitchDenyAuthorizer struct{}

func (hgSpiderSwitchDenyAuthorizer) HasAssetPermission(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestHGSpiderSwitchRejectsUnauthenticatedAndUnprivileged(t *testing.T) {
	store := &hgSpiderSwitchStoreStub{}
	handler := NewHandler(nil).WithCrawlerTasks(nil, hgSpiderSwitchDenyAuthorizer{}).WithCrawlerSpiderSwitch(store)
	recorder := httptest.NewRecorder()
	handler.HGCrawlerSpiderStart(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/ops/crawler/spider/start", nil))
	if recorder.Code != http.StatusUnauthorized || store.called {
		t.Fatalf("未登录不能切换: status=%d called=%t", recorder.Code, store.called)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ops/crawler/spider/start", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserJWTMiddlewarePackage.UserIDKey, &UserServicePackage.HGClaims{UserID: "admin"}))
	recorder = httptest.NewRecorder()
	handler.HGCrawlerSpiderStart(recorder, req)
	if recorder.Code != http.StatusForbidden || store.called {
		t.Fatalf("无权限不能切换: status=%d called=%t", recorder.Code, store.called)
	}
}

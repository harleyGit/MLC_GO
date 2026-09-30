package HGRouterPackage

import (
	hguser "MLC_GO/internal/modules/user/service"
	"MLC_GO/internal/modules/wallet/handler"
	hgserver "MLC_GO/internal/pkg/server"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHGWalletCatalogAndGuard(t *testing.T) {
	hgHandler := handler.HGNewHandler(nil)
	hgRoot := http.NewServeMux()
	hgRoot.Handle(HGWalletModuleBasePath+"/", http.StripPrefix(HGWalletModuleBasePath, HGNewWalletRouteGroup(hgHandler)))
	if len(HGWalletRouteCatalog()) != 5 {
		t.Fatal("钱包目录必须完整")
	}
	for _, hgRoute := range hgWalletRoutes(hgHandler) {
		hgFound := false
		for _, hgRule := range hgserver.HGWalletMethodRules() {
			if hgRule.Path == hgRoute.SubPath && hgRule.Version == "v1" && hgRule.Methods[hgRoute.Method] && hgRule.NeedAuth {
				hgFound = true
			}
		}
		if !hgFound || !hgRoute.NeedAuth || hgRoute.Handler == nil {
			t.Fatalf("路由和Guard不一致: %+v", hgRoute)
		}
		for _, hgCase := range []struct {
			hgMethod, hgVersion string
			hgHeaders           bool
			hgStatus            int
		}{
			{hgRoute.Method, "v1", false, 400}, {hgRoute.Method, "v1", true, 401}, {http.MethodDelete, "v1", false, 405}, {hgRoute.Method, "v999", false, 404},
		} {
			hgRequest := httptest.NewRequest(hgCase.hgMethod, hgRoute.FullPath, nil)
			hgRequest.Header.Set("X-API-Version", hgCase.hgVersion)
			if hgCase.hgHeaders {
				for _, hgHeader := range []string{"Content-Type", "X-Device-ID", "X-Client-Type", "X-Client-Version", "X-Language", "X-Request-ID", "X-Timestamp", "X-Signature"} {
					hgRequest.Header.Set(hgHeader, "test-placeholder")
				}
			}
			hgRecorder := httptest.NewRecorder()
			hgRoot.ServeHTTP(hgRecorder, hgRequest)
			if hgRecorder.Code != hgCase.hgStatus {
				t.Fatalf("%s status=%d body=%s", hgRoute.FullPath, hgRecorder.Code, hgRecorder.Body.String())
			}
		}
	}
}

// TestHGWalletSignaturePath uses the real wallet guard and JWT chain without a service or external I/O.
func TestHGWalletSignaturePath(t *testing.T) {
	hgRoot := http.StripPrefix(HGWalletModuleBasePath, HGNewWalletRouteGroup(handler.HGNewHandler(nil)))
	for _, hgCase := range []struct {
		hgName, hgPath, hgQuery string
		hgSignatureFailure      bool
	}{
		{"old full path", "/api/v1/wallet/recharge/skus", "?pageSize=20&cursor=0", true},
		{"module path", "/recharge/skus", "?pageSize=20&cursor=0", false},
		{"reordered query", "/recharge/skus", "?cursor=0&pageSize=20", false},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgReq := httptest.NewRequest(http.MethodGet, HGWalletModuleBasePath+"/recharge/skus"+hgCase.hgQuery, nil)
			hgHeaders := map[string]string{
				"Content-Type": "application/json", "Authorization": "Bearer test-invalid-jwt",
				"X-Timestamp": strconv.FormatInt(time.Now().Unix(), 10), "X-Request-ID": "test-request",
				"X-Device-ID": "test-device", "X-Client-Type": "web", "X-Client-Version": "1",
				"X-API-Version": "v1", "X-Language": "zh-CN",
			}
			for hgKey, hgValue := range hgHeaders {
				hgReq.Header.Set(hgKey, hgValue)
			}
			// Mirror HttpManagerV1's canonical payload: pathname only, empty GET body and Authorization last.
			hgBodyHash := sha256.Sum256(nil)
			hgPayload := strings.Join([]string{http.MethodGet, hgCase.hgPath,
				hgHeaders["X-Timestamp"], hgHeaders["X-Request-ID"], hgHeaders["X-Device-ID"],
				hgHeaders["X-Client-Type"], hgHeaders["X-Client-Version"], hgHeaders["X-API-Version"],
				hgHeaders["X-Language"], hex.EncodeToString(hgBodyHash[:]), hgHeaders["Authorization"]}, "\n")
			hgMAC := hmac.New(sha256.New, hguser.Secret)
			_, _ = hgMAC.Write([]byte(hgPayload))
			hgReq.Header.Set("X-Signature", "sha256="+hex.EncodeToString(hgMAC.Sum(nil)))
			hgRec := httptest.NewRecorder()
			hgRoot.ServeHTTP(hgRec, hgReq)
			var hgResponse struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			if hgErr := json.Unmarshal(hgRec.Body.Bytes(), &hgResponse); hgErr != nil {
				t.Fatalf("expected one JSON: %v", hgErr)
			}
			if hgRec.Code != http.StatusUnauthorized || hgResponse.Code != 300001 {
				t.Fatalf("unexpected status/code: %d/%d", hgRec.Code, hgResponse.Code)
			}
			if hgCase.hgSignatureFailure {
				if hgResponse.Message != "signature无效" {
					t.Fatal("full path signature must fail")
				}
			} else if !strings.HasPrefix(hgResponse.Message, "Token无效") {
				t.Fatal("valid module signature must reach JWT rejection, never bypass authentication")
			}
		})
	}
}

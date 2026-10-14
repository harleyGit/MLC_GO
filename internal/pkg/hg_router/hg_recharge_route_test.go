package HGRouterPackage

import (
	"MLC_GO/internal/modules/recharge/handler"
	hgserver "MLC_GO/internal/pkg/server"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHGRechargeCatalogGuardAndMount(t *testing.T) {
	if len(HGRechargeRouteCatalog()) != 1 || HGRechargeRouteCatalog()[0].Path != "/api/v1/recharge/records" {
		t.Fatal("recharge catalog missing")
	}
	hgRoot := http.NewServeMux()
	hgRoot.Handle(HGRechargeModuleBasePath+"/", http.StripPrefix(HGRechargeModuleBasePath, HGNewRechargeRouteGroup(handler.HGNewHandler(nil))))
	hgRoutes := hgRechargeRoutes(handler.HGNewHandler(nil))
	for _, hgRoute := range hgRoutes {
		var hgFound bool
		for _, hgRule := range hgserver.HGRechargeMethodRules() {
			if hgRule.Path == hgRoute.SubPath && hgRule.Methods[hgRoute.Method] && hgRule.NeedAuth {
				hgFound = true
			}
		}
		if !hgFound || !hgRoute.NeedAuth || hgRoute.Handler == nil {
			t.Fatalf("route/guard mismatch: %+v", hgRoute)
		}
		hgRequest := httptest.NewRequest(hgRoute.Method, hgRoute.FullPath, nil)
		hgRequest.Header.Set("X-API-Version", "v1")
		hgRecorder := httptest.NewRecorder()
		hgRoot.ServeHTTP(hgRecorder, hgRequest)
		if hgRecorder.Code != http.StatusBadRequest {
			t.Fatalf("guard status=%d body=%s", hgRecorder.Code, hgRecorder.Body.String())
		}
	}
}

// TestHGRechargeSignaturePath 使用真实 StripPrefix、签名守卫及 JWT 链验证充值签名契约，无外部 I/O。
func TestHGRechargeSignaturePath(t *testing.T) {
	hgRoot := http.StripPrefix(HGRechargeModuleBasePath, HGNewRechargeRouteGroup(handler.HGNewHandler(nil)))
	for _, hgCase := range []struct {
		hgName          string // 用例名称，区分路径错误、合法签名及缺失认证。
		hgSignPath      string // 前端用于签名的 pathname，不包含查询参数。
		hgQuery         string // 实际请求查询串，遵循既有不参与签名的协议。
		hgAuthorization string // 使用无效测试 JWT，确保请求在进入业务层前结束。
		hgMessagePrefix string // 预期错误前缀，用来区分签名守卫和 JWT 拒绝。
	}{
		{"完整路径签名被拒绝", HGRechargeModuleBasePath + "/records", "?pageSize=50", "Bearer fixture-invalid-jwt", "signature无效"},
		{"模块路径签名通过", "/records", "?pageSize=50", "Bearer fixture-invalid-jwt", "Token无效"},
		{"查询参数不参与签名", "/records", "?cursor=fixture&pageSize=50", "Bearer fixture-invalid-jwt", "Token无效"},
		{"缺少认证仍被拒绝", "/records", "?pageSize=50", "", "Authorization不能为空"},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgReq := httptest.NewRequest(http.MethodGet, HGRechargeModuleBasePath+"/records"+hgCase.hgQuery, nil)
			for hgKey, hgValue := range map[string]string{
				"Content-Type": "application/json", "Authorization": hgCase.hgAuthorization,
				"X-Timestamp": strconv.FormatInt(time.Now().Unix(), 10), "X-Request-ID": "fixture-request",
				"X-Device-ID": "fixture-device", "X-Client-Type": "web", "X-Client-Version": "1",
				"X-API-Version": "v1", "X-Language": "zh-CN",
			} {
				hgReq.Header.Set(hgKey, hgValue)
			}
			// 复用既有空 GET body 签名夹具，仅替换参与签名的路径。
			hgSigned := hgReq.Clone(hgReq.Context())
			hgSigned.URL.Path = hgCase.hgSignPath
			hgReq.Header.Set("X-Signature", buildSignature(hgSigned))
			hgRecorder := httptest.NewRecorder()
			hgRoot.ServeHTTP(hgRecorder, hgReq)
			var hgResponse struct {
				Code    int    `json:"code"`    // 统一响应业务错误码。
				Message string `json:"message"` // 守卫或认证错误说明。
			}
			if hgErr := json.Unmarshal(hgRecorder.Body.Bytes(), &hgResponse); hgErr != nil {
				t.Fatalf("响应应为单个 JSON: %v", hgErr)
			}
			if hgRecorder.Code != http.StatusUnauthorized || hgResponse.Code != 300001 || !strings.HasPrefix(hgResponse.Message, hgCase.hgMessagePrefix) {
				t.Fatalf("错误响应不符: status=%d code=%d message=%q", hgRecorder.Code, hgResponse.Code, hgResponse.Message)
			}
		})
	}
}

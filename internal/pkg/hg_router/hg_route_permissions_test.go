package HGRouterPackage

import (
	OpsHandlerPackage "MLC_GO/internal/modules/ops/handler"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	HGMiddlewarePackage "MLC_GO/internal/pkg/middleware"
	HGServerPackage "MLC_GO/internal/pkg/server"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type hgRouteAuthorizerStub struct {
	hgAllowed map[string]bool // 本用例允许的权限编码。
	hgErr     error           // 模拟权限存储故障。
	hgCalls   []string        // 实际授权查询，确认拒绝不触发业务。
	hgActor   string          // JWT 产生的可信用户标识。
}

func (hgStub *hgRouteAuthorizerStub) HasAssetPermission(hgCtx context.Context, hgUser, hgPermission string) (bool, error) {
	hgStub.hgCalls = append(hgStub.hgCalls, hgPermission)
	hgStub.hgActor = hgUser
	if _, hgOK := hgCtx.Deadline(); !hgOK {
		return false, errors.New("权限查询缺少超时")
	}
	return hgStub.hgAllowed[hgPermission], hgStub.hgErr
}

// hgSignedPermissionRequest 走根入口相同的 StripPrefix 链；签名按模块内路径计算。
func hgSignedPermissionRequest(t *testing.T, hgMethod, hgPath, hgTokenMode string) *http.Request {
	t.Helper()
	hgRequest := httptest.NewRequest(hgMethod, "/api/v1/test"+hgPath, nil)
	for hgKey, hgValue := range map[string]string{
		"Content-Type": "application/json", "X-Device-ID": "permission-test", "X-Client-Type": "ios",
		"X-Client-Version": "1.0.0", "X-API-Version": "v1", "X-Language": "zh-CN",
		"Accept-Language": "zh-CN", "X-Request-ID": "permission-test-request", "User-Agent": "permission-test",
		"X-Timestamp": strconv.FormatInt(time.Now().Unix(), 10), "X-Role": "super_admin", "X-User-ID": "forged-user",
	} {
		hgRequest.Header.Set(hgKey, hgValue)
	}
	if hgTokenMode == "valid" {
		hgClaims := &UserServicePackage.HGClaims{UserID: "trusted-user", TokenTp: "access", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)), Issuer: "mlc-go", Subject: "user-token"}}
		hgToken, hgErr := jwt.NewWithClaims(jwt.SigningMethodHS256, hgClaims).SignedString(UserServicePackage.Secret)
		if hgErr != nil {
			t.Fatal(hgErr)
		}
		hgRequest.Header.Set("Authorization", "Bearer "+hgToken)
	} else if hgTokenMode == "invalid" {
		hgRequest.Header.Set("Authorization", "Bearer forged-token")
	}
	hgInner := hgRequest.Clone(hgRequest.Context())
	hgInner.URL.Path = hgPath
	hgRequest.Header.Set("X-Signature", buildSignature(hgInner))
	return hgRequest
}

func TestHGRoutePermissionChain(t *testing.T) {
	for _, hgCase := range []struct {
		hgName    string          // 用例说明。
		hgToken   string          // valid、invalid 或空。
		hgAllowed map[string]bool // IAM 查询结果。
		hgErr     error           // 模拟存储错误。
		hgMissing bool            // 是否故意漏注入授权器。
		hgAny     bool            // 是否为任一权限匹配。
		hgStatus  int             // 期望 HTTP 状态。
	}{
		{hgName: "无JWT", hgStatus: 401},
		{hgName: "伪造JWT和角色头", hgToken: "invalid", hgStatus: 401},
		{hgName: "无权限且伪造角色头", hgToken: "valid", hgStatus: 403},
		{hgName: "复合权限只满足一项", hgToken: "valid", hgAllowed: map[string]bool{"crawler:task:write": true}, hgStatus: 403},
		{hgName: "全部满足", hgToken: "valid", hgAllowed: map[string]bool{"crawler:task:write": true, "crawler:task:run": true}, hgStatus: 204},
		{hgName: "任一满足", hgToken: "valid", hgAllowed: map[string]bool{"crawler:task:run": true}, hgAny: true, hgStatus: 204},
		{hgName: "任一均不满足", hgToken: "valid", hgAny: true, hgStatus: 403},
		{hgName: "依赖错误不泄露", hgToken: "valid", hgErr: errors.New("private-database-diagnostic"), hgStatus: 503},
		{hgName: "漏注入默认拒绝", hgToken: "valid", hgMissing: true, hgStatus: 503},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgCalled := false
			hgSpec := NewRouteSpec("test", http.MethodPost, "/api/v1/test", "/run", true, "复合操作", func(w http.ResponseWriter, r *http.Request) { hgCalled = true; w.WriteHeader(http.StatusNoContent) }, "crawler:task:write", "crawler:task:run")
			if hgCase.hgAny {
				hgSpec = hgSpec.HGWithAnyPermission()
			}
			hgStub := &hgRouteAuthorizerStub{hgAllowed: hgCase.hgAllowed, hgErr: hgCase.hgErr}
			hgConfig := RouteGroupConfig{Rules: []HGMiddlewarePackage.APIRule{{Path: "/run", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true}}, AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware, Authorizer: hgStub}
			if hgCase.hgMissing {
				hgConfig.Authorizer = nil
			}
			hgRoot := http.NewServeMux()
			hgRoot.Handle("/api/v1/test/", http.StripPrefix("/api/v1/test", NewRouteGroup(hgConfig, []RouteSpec{hgSpec})))
			hgResponse := httptest.NewRecorder()
			hgRoot.ServeHTTP(hgResponse, hgSignedPermissionRequest(t, http.MethodPost, "/run", hgCase.hgToken))
			if hgResponse.Code != hgCase.hgStatus || hgCalled != (hgCase.hgStatus == 204) {
				t.Fatalf("状态=%d 业务执行=%v 响应=%s", hgResponse.Code, hgCalled, hgResponse.Body.String())
			}
			if hgCase.hgStatus == 401 && len(hgStub.hgCalls) > 0 {
				t.Fatal("JWT 失败仍查询了权限")
			}
			if len(hgStub.hgCalls) > 0 && hgStub.hgActor != "trusted-user" {
				t.Fatal("授权未使用 JWT 用户")
			}
			if strings.Contains(hgResponse.Body.String(), "private-database-diagnostic") {
				t.Fatal("泄露存储诊断")
			}
		})
	}
}

func TestHGRoutePermissionsCatalogAndBinding(t *testing.T) {
	hgCatalog := BuildRouteCatalogItems(opsRoutes(nil))
	hgBound := BuildRouteCatalogItems(opsRoutes(&OpsHandlerPackage.Handler{}))
	if !reflect.DeepEqual(hgCatalog, hgBound) {
		t.Fatal("运维目录与实际绑定权限不同")
	}
	hgSeeds := ""
	for _, hgFile := range []string{"000046_create_iam.up.sql", "000047_iam_cutover_security.up.sql", "000048_route_permissions.up.sql"} {
		hgBytes, hgErr := os.ReadFile("../../../migrations/" + hgFile)
		if hgErr != nil {
			t.Fatal(hgErr)
		}
		hgSeeds += string(hgBytes)
	}
	for _, hgItem := range hgCatalog {
		if len(hgItem.Permissions) == 0 && hgItem.Path != OpsModuleBasePath+"/asset-permissions/current" {
			t.Errorf("运维接口未声明权限: %s", hgItem.Path)
		}
		for _, hgPermission := range hgItem.Permissions {
			if !strings.Contains(hgSeeds, "'"+hgPermission+"'") {
				t.Errorf("权限未登记迁移: %s", hgPermission)
			}
		}
	}
	hgSpecs := opsRoutes(nil)
	hgCatalog = BuildRouteCatalogItems(hgSpecs)
	hgCatalog[0].Permissions[0] = "changed"
	if hgSpecs[0].Permissions[0] == "changed" {
		t.Fatal("权限目录共享可变切片")
	}
	_ = hgRouteGuardRules(HGServerPackage.OpsMethodRules(), hgSpecs)
}

func TestHGStandaloneGuardDoesNotTrustRoleHeader(t *testing.T) {
	hgCalled := false
	hgGuard := HGMiddlewarePackage.APIGuardMiddleware([]HGMiddlewarePackage.APIRule{{Path: "/run", Version: "v1", NeedAuth: true, Methods: map[string]bool{http.MethodPost: true}, Permissions: []string{"user:update"}}})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hgCalled = true }))
	hgRequest := hgSignedPermissionRequest(t, http.MethodPost, "/run", "valid")
	hgRequest.URL.Path = "/run"
	hgRequest.Header.Set("X-Role", "admin")
	hgResponse := httptest.NewRecorder()
	hgGuard.ServeHTTP(hgResponse, hgRequest)
	if hgResponse.Code != 403 || hgCalled {
		t.Fatalf("伪造角色头被接受: %d", hgResponse.Code)
	}
}

// 覆盖全部主业务路由组，保证认证声明与 Guard 一致；不启动服务器或访问外部依赖。
func TestHGAllRouteGroupPermissionContracts(t *testing.T) {
	NewAuthRouteGroup(nil)
	NewUserRouteGroup(nil)
	NewOpsRouteGroup(nil)
	NewVideoUploadRouteGroup(nil)
	NewVideoInteractionRouteGroup(nil)
	NewVideoCommentRouteGroup(nil)
	NewVideoDanmakuRouteGroup(nil)
	NewBilibiliRouteGroup(nil)
	NewVideoRecommendRouteGroup(nil)
	HGNewMallRouteGroup(nil)
	HGNewWalletRouteGroup(nil)
	HGNewRechargeRouteGroup(nil)
}

func TestHGRoutePermissionConfigurationRejectsInvalidDeclarations(t *testing.T) {
	hgBase := NewRouteSpec("test", http.MethodGet, "/test", "/list", true, "查询", nil, "iam:role:read")
	for _, hgCase := range []string{"匿名权限", "非法编码", "重复权限", "未知匹配", "空列表匹配", "超量权限", "Guard认证漂移", "Guard权限漂移", "Guard遗漏"} {
		t.Run(hgCase, func(t *testing.T) {
			hgSpec := hgBase
			hgRules := []HGMiddlewarePackage.APIRule{{Path: "/list", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true}}
			switch hgCase {
			case "匿名权限":
				hgSpec.NeedAuth = false
			case "非法编码":
				hgSpec.Permissions = []string{"iam.role.read"}
			case "重复权限":
				hgSpec.Permissions = []string{"iam:role:read", "iam:role:read"}
			case "未知匹配":
				hgSpec.PermissionMatch = "unknown"
			case "空列表匹配":
				hgSpec.Permissions = nil
			case "超量权限":
				hgSpec.Permissions = make([]string, 17)
			case "Guard认证漂移":
				hgRules[0].NeedAuth = false
			case "Guard权限漂移":
				hgRules[0].Permissions = []string{"iam:role:grant"}
			case "Guard遗漏":
				hgRules[0].Path = "/other"
			}
			defer func() {
				if recover() == nil {
					t.Fatal("无效权限配置未阻止启动")
				}
			}()
			NewRouteGroup(RouteGroupConfig{Rules: hgRules}, []RouteSpec{hgSpec})
		})
	}
}

func TestHGMethodBoundPermissionsAndPublicRoutes(t *testing.T) {
	hgCalled := 0
	hgHandler := func(w http.ResponseWriter, r *http.Request) { hgCalled++; w.WriteHeader(204) }
	hgMux := http.NewServeMux()
	hgStub := &hgRouteAuthorizerStub{hgAllowed: map[string]bool{"ops:file:read": true}}
	BindRouteSpecs(hgMux, []RouteSpec{
		NewRouteSpec("test", http.MethodGet, "", "/files", false, "公开读", hgHandler),
		NewRouteSpec("test", http.MethodPost, "", "/files", true, "受限写", hgHandler, "ops:file:write"),
	}, hgStub)
	for _, hgMethod := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		hgResponse := httptest.NewRecorder()
		hgRequest := httptest.NewRequest(hgMethod, "/files", nil)
		if hgMethod == http.MethodPost {
			hgRequest = hgRequest.WithContext(context.WithValue(hgRequest.Context(), UserJWTMiddlewarePackage.UserIDKey, &UserServicePackage.HGClaims{UserID: "reader"}))
		}
		hgMux.ServeHTTP(hgResponse, hgRequest)
		hgWant := map[string]int{http.MethodGet: 204, http.MethodPost: 403, http.MethodDelete: 405}[hgMethod]
		if hgResponse.Code != hgWant {
			t.Fatalf("%s 状态=%d 预期=%d", hgMethod, hgResponse.Code, hgWant)
		}
	}
	if hgCalled != 1 {
		t.Fatal("未授权方法进入了业务处理")
	}
}

func TestHGOpsLegacyRoleRouteStillGoneAfterAuthorization(t *testing.T) {
	hgStub := &hgRouteAuthorizerStub{hgAllowed: map[string]bool{"iam:role:read": true}}
	hgRoot := http.NewServeMux()
	hgRoot.Handle(OpsModuleBasePath+"/", http.StripPrefix(OpsModuleBasePath, NewOpsRouteGroup(&OpsHandlerPackage.Handler{}, hgStub)))
	hgRequest := hgSignedPermissionRequest(t, http.MethodGet, "/roles/list", "valid")
	hgRequest.URL.Path = OpsModuleBasePath + "/roles/list"
	hgResponse := httptest.NewRecorder()
	hgRoot.ServeHTTP(hgResponse, hgRequest)
	if hgResponse.Code != 410 {
		t.Fatalf("旧角色入口应继续提示迁移: %d %s", hgResponse.Code, hgResponse.Body.String())
	}
}

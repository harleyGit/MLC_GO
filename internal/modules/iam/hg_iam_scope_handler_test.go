package iam

import (
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// hgScopeHTTPRequest 仅注入测试身份，覆盖真实 JSON 解码和业务写入，不访问外部服务。
func hgScopeHTTPRequest(hgPath, hgBody, hgActorID string) *http.Request {
	hgRequest := httptest.NewRequest(http.MethodPost, "/api/v1/iam/scope-types"+hgPath, strings.NewReader(hgBody))
	hgRequest.Header.Set("X-Request-ID", "request-1")
	if hgActorID != "" {
		hgRequest = hgRequest.WithContext(context.WithValue(hgRequest.Context(), UserJWTMiddlewarePackage.UserIDKey, &UserServicePackage.HGClaims{UserID: hgActorID}))
	}
	return hgRequest
}

func hgExpectScopeManager(hgMock sqlmock.Sqlmock, hgAllowed bool) {
	hgMock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(hgMock, "ACTOR", "global", "", "iam:role:grant", hgAllowed)
}

// 创建必须把客户端状态保存并审计；旧请求省略/null 保持默认停用。
func TestHGScopeCreateStatusHTTP(t *testing.T) {
	for _, hgCase := range []struct {
		hgName   string // 场景名称。
		hgField  string // 可选 JSON status 字段，空串表示旧请求。
		hgStatus int    // 预期持久化的 0/1 状态。
	}{
		{"启用", `,"status":1`, 1},
		{"停用", `,"status":0`, 0},
		{"省略", "", 0},
		{"空值", `,"status":null`, 0},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgService, hgMock := hgTestService(t)
			hgExpectScopeManager(hgMock, true)
			hgMock.ExpectBegin()
			hgMock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMScopeType)).WithArgs("new_scope", "新类型", "用途", true, hgCase.hgStatus).WillReturnResult(sqlmock.NewResult(10, 1))
			hgMock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).WithArgs("ACTOR", "iam.scope_type.create", "scope_type", "new_scope", "request-1", []byte(fmt.Sprintf(`{"status":%d}`, hgCase.hgStatus))).WillReturnResult(sqlmock.NewResult(1, 1))
			hgMock.ExpectCommit()
			hgResponse := httptest.NewRecorder()
			NewHGHandler(hgService).HGCreateScopeType(hgResponse, hgScopeHTTPRequest("", `{"code":"new_scope","name":"新类型","description":"用途"`+hgCase.hgField+`}`, "ACTOR"))
			var hgResult struct {
				Result map[string]bool `json:"result"` // 创建接口成功标记。
			}
			if hgErr := json.Unmarshal(hgResponse.Body.Bytes(), &hgResult); hgErr != nil {
				t.Fatal(hgErr)
			}
			if hgResponse.Code != http.StatusOK || !hgResult.Result["created"] {
				t.Fatalf("创建失败: status=%d body=%s", hgResponse.Code, hgResponse.Body.String())
			}
			if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

// 编辑的启停、引用保护和审计必须处于同一事务，省略状态则保留锁定后的值。
func TestHGScopeUpdateStatusHTTP(t *testing.T) {
	for _, hgCase := range []struct {
		hgName       string // 场景名称。
		hgField      string // 可选 status 字段。
		hgPrevious   int    // 锁行读取的旧状态。
		hgStatus     int    // 预期新状态。
		hgReferenced bool   // 是否存在历史绑定。
		hgAuditFails bool   // 审计失败时必须回滚。
	}{
		{"启用", `,"status":1`, 0, 1, false, false},
		{"停用", `,"status":0`, 1, 0, false, false},
		{"保持停用", `,"status":0`, 0, 0, false, false},
		{"旧请求保留启用", "", 1, 1, false, false},
		{"旧请求保留停用", "", 0, 0, false, false},
		{"空值保留状态", `,"status":null`, 1, 1, false, false},
		{"有引用禁止停用", `,"status":0`, 1, 0, true, false},
		{"审计失败回滚启用", `,"status":1`, 0, 1, false, true},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgService, hgMock := hgTestService(t)
			hgExpectScopeManager(hgMock, true)
			hgMock.ExpectBegin()
			hgMock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeTypeForUpdate)).WithArgs("region").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(hgCase.hgPrevious))
			if hgCase.hgField == `,"status":0` {
				hgRows := sqlmock.NewRows([]string{"one"})
				if hgCase.hgReferenced {
					hgRows.AddRow(1)
				}
				hgMock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeBinding)).WithArgs("region").WillReturnRows(hgRows)
			}
			hgWantHTTP := http.StatusOK
			if hgCase.hgReferenced {
				hgWantHTTP = http.StatusConflict
				hgMock.ExpectRollback()
			} else {
				hgMock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGUpdateIAMScopeType)).WithArgs("区域", "用途", hgCase.hgStatus, "region").WillReturnResult(sqlmock.NewResult(0, 1))
				hgAudit := hgMock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).WithArgs("ACTOR", "iam.scope_type.update", "scope_type", "region", "request-1", []byte(fmt.Sprintf(`{"name":"区域","previousStatus":%d,"status":%d}`, hgCase.hgPrevious, hgCase.hgStatus)))
				if hgCase.hgAuditFails {
					hgAudit.WillReturnError(errors.New("审计不可用"))
					hgMock.ExpectRollback()
					hgWantHTTP = http.StatusInternalServerError
				} else {
					hgAudit.WillReturnResult(sqlmock.NewResult(1, 1))
					hgMock.ExpectCommit()
				}
			}
			hgResponse := httptest.NewRecorder()
			NewHGHandler(hgService).HGUpdateScopeType(hgResponse, hgScopeHTTPRequest("/update", `{"code":"region","name":"区域","description":"用途"`+hgCase.hgField+`}`, "ACTOR"))
			if hgResponse.Code != hgWantHTTP {
				t.Fatalf("更新状态错误: status=%d body=%s", hgResponse.Code, hgResponse.Body.String())
			}
			if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

func TestHGScopeStatusRejectsInvalidAndUnauthorizedHTTP(t *testing.T) {
	for _, hgPath := range []string{"", "/update"} {
		for _, hgCase := range []struct {
			hgName    string // 场景名称。
			hgValue   string // JSON 状态值。
			hgActorID string // 空字符串表示未登录。
			hgDecoded bool   // JSON 类型通过后才查询权限。
			hgAllowed bool   // 当前身份是否拥有管理权限。
			hgHTTP    int    // 预期 HTTP 错误码。
		}{
			{"负值", "-1", "ACTOR", true, true, 400},
			{"超范围", "2", "ACTOR", true, true, 400},
			{"布尔", "true", "ACTOR", false, true, 400},
			{"字符串", `"1"`, "ACTOR", false, true, 400},
			{"小数", "0.5", "ACTOR", false, true, 400},
			{"未登录", "1", "", false, false, 401},
			{"无权限", "1", "ACTOR", true, false, 403},
		} {
			t.Run(hgPath+hgCase.hgName, func(t *testing.T) {
				hgService, hgMock := hgTestService(t)
				if hgCase.hgDecoded {
					hgExpectScopeManager(hgMock, hgCase.hgAllowed)
				}
				hgHandler := NewHGHandler(hgService).HGCreateScopeType
				if hgPath != "" {
					hgHandler = NewHGHandler(hgService).HGUpdateScopeType
				}
				hgResponse := httptest.NewRecorder()
				hgHandler(hgResponse, hgScopeHTTPRequest(hgPath, `{"code":"region","name":"区域","status":`+hgCase.hgValue+`}`, hgCase.hgActorID))
				if hgResponse.Code != hgCase.hgHTTP {
					t.Fatalf("非法请求未被正确拒绝: status=%d body=%s", hgResponse.Code, hgResponse.Body.String())
				}
				if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
					t.Fatal(hgErr)
				}
			})
		}
	}
}

func TestHGScopeUpdateCannotDisableCoreHTTP(t *testing.T) {
	for _, hgCode := range []string{"global", "shop", "org"} {
		t.Run(hgCode, func(t *testing.T) {
			hgService, hgMock := hgTestService(t)
			hgExpectScopeManager(hgMock, true)
			hgResponse := httptest.NewRecorder()
			NewHGHandler(hgService).HGUpdateScopeType(hgResponse, hgScopeHTTPRequest("/update", fmt.Sprintf(`{"code":%q,"name":"核心类型","status":0}`, hgCode), "ACTOR"))
			if hgResponse.Code != http.StatusForbidden {
				t.Fatalf("核心类型停用未被拒绝: status=%d", hgResponse.Code)
			}
			if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

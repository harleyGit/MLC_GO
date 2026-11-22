package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 候选查询同时要求活动管理员和即时授权权限；无权时不读取组织目录。
func TestHGListScopeResourcesRejectsWithoutDirectGrant(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", false)
	if _, _, _, err := service.HGListScopeResources(context.Background(), "ACTOR", "org", "", 50); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("无即时授权权限应拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 组织及已审核商家只按索引倒序游标分页，续页游标必须对应实际返回的最后一项。
func TestHGListScopeResourcesPages(t *testing.T) {
	for _, tc := range []struct {
		name, scope, cursor, query, next, firstID string
		args                                      []driver.Value
		rows                                      *sqlmock.Rows
	}{
		{"org", "org", "org-9", SQLQueriesPackage.HGSelectIAMScopeOrgsAfter, "org-7", "org-8",
			[]driver.Value{"org", "org-9", 3}, sqlmock.NewRows([]string{"id", "name"}).AddRow("org-8", "八号组织").AddRow("org-7", "七号组织").AddRow("org-6", "六号组织")},
		{"shop", "shop", "m:9", SQLQueriesPackage.HGSelectIAMScopeShopsAfter, "m:7", "MER-8",
			[]driver.Value{uint64(9), 3}, sqlmock.NewRows([]string{"id", "merchant_id", "shop_name"}).AddRow(8, "MER-8", "八号店铺").AddRow(7, "MER-7", "七号店铺").AddRow(6, "MER-6", "六号店铺")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, mock := hgTestService(t)
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
			hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
			hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
			mock.ExpectQuery(regexp.QuoteMeta(tc.query)).WithArgs(tc.args...).WillReturnRows(tc.rows)
			items, next, more, err := service.HGListScopeResources(context.Background(), "ACTOR", tc.scope, tc.cursor, 2)
			if err != nil || !more || next != tc.next || len(items) != 2 || items[0].ID != tc.firstID {
				t.Fatalf("分页响应不正确: items=%+v next=%q more=%v err=%v", items, next, more, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// 已登记 shop 优先展示；跨来源翻页时继续读取已审核商家，且不重复列出同 ID。
func TestHGListScopeResourcesRegisteredShop(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeOrgsFirst)).WithArgs("shop", 3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("shop-1", "已登记店铺"))
	items, next, more, err := service.HGListScopeResources(context.Background(), "ACTOR", "shop", "", 2)
	if err != nil || len(items) != 1 || items[0].ID != "shop-1" || next != "m:0" || !more {
		t.Fatalf("登记店铺候选不正确: items=%+v next=%q more=%v err=%v", items, next, more, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 停用或未接通资源校验的类型不得通过候选查询绕过授权 allowlist。
func TestHGListScopeResourcesRejectsUnsupportedType(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	if _, _, _, err := service.HGListScopeResources(context.Background(), "ACTOR", "team", "", 20); !errors.Is(err, ErrHGInvalid) {
		t.Fatalf("未接通的作用域类型应拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 六类资源 ID 必须存在于启用的 IAM 登记表；不允许用伪造 ID 或类型绕过权威点查。
func TestHGRegisteredScopeRequiresActiveResource(t *testing.T) {
	service, mock := hgTestService(t)
	for _, scopeType := range []string{"creator", "organization", "environment", "crawler_project", "content_channel", "region"} {
		mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeResourceActive)).
			WithArgs(scopeType, "resource-1").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
		active, err := service.repo.hgOrganization(context.Background(), scopeType, "resource-1")
		if err != nil || !active {
			t.Fatalf("%s 有效登记资源应可用: %v", scopeType, err)
		}
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeResourceActive)).
		WithArgs("environment", "unknown").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	active, err := service.repo.hgOrganization(context.Background(), "environment", "unknown")
	if err != nil || active {
		t.Fatalf("不存在的环境不得用于授权: active=%v err=%v", active, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 新作用域候选资源按服务端主键倒序游标分页，不使用深 offset 或无界目录。
func TestHGListRegisteredScopeResourcesPage(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMReadableScopeType)).
		WithArgs("environment").WillReturnRows(sqlmock.NewRows([]string{"requires_scope_id"}).AddRow(true))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeResourcesAfter)).
		WithArgs("environment", uint64(9), 3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "scope_id", "scope_name"}).
			AddRow(8, "env-a", "环境 A").AddRow(7, "env-b", "环境 B").AddRow(6, "env-c", "环境 C"))
	items, next, more, err := service.HGListScopeResources(context.Background(), "ACTOR", "environment", "9", 2)
	if err != nil || len(items) != 2 || items[0].ID != "env-a" || next != "7" || !more {
		t.Fatalf("候选页异常: items=%+v next=%q more=%v err=%v", items, next, more, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 登记接口不接受客户端自行指定 ID，事务内锁定启用类型并写入审计，名称不能为空。
func TestHGCreateScopeResourceAudited(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeResourceRequest)).
		WithArgs("ACTOR", "request-1").WillReturnRows(sqlmock.NewRows([]string{"scope_id", "scope_type", "scope_name"}))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMActiveScopeType)).
		WithArgs("region").WillReturnRows(sqlmock.NewRows([]string{"requires_scope_id"}).AddRow(true))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMScopeResource)).
		WithArgs("region", sqlmock.AnyArg(), "华北", "ACTOR", "request-1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).
		WithArgs("ACTOR", "iam.scope_resource.create", "scope_resource", sqlmock.AnyArg(), "request-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	item, err := service.HGCreateScopeResource(context.Background(), "ACTOR", "request-1", "region", " 华北 ")
	if err != nil || len(item.ID) != 32 || item.Name != "华北" {
		t.Fatalf("登记响应不正确: item=%+v err=%v", item, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 同一管理员请求 ID 重试复用已有资源，参数改变则拒绝，不能重复登记或重新审计。
func TestHGCreateScopeResourceReplay(t *testing.T) {
	service, mock := hgTestService(t)
	for range 2 {
		mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").
			WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
		hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
		hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
		mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeResourceRequest)).
			WithArgs("ACTOR", "request-1").WillReturnRows(sqlmock.NewRows([]string{"scope_id", "scope_type", "scope_name"}).
			AddRow("registered-id", "region", "华北"))
	}
	item, err := service.HGCreateScopeResource(context.Background(), "ACTOR", "request-1", "region", "华北")
	if err != nil || item.ID != "registered-id" {
		t.Fatalf("相同请求未复用资源: item=%+v err=%v", item, err)
	}
	if _, err = service.HGCreateScopeResource(context.Background(), "ACTOR", "request-1", "environment", "华北"); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("不同参数复用请求应冲突: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 新作用域在业务服务传入资源后须先从登记表核对 ID；不存在时即使全局授权存在也拒绝。
func TestHGAuthorizeRejectsUnknownRegisteredScope(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeResourceActive)).
		WithArgs("region", "unknown").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	decision, err := service.Authorize(context.Background(), "ACTOR", "mall:product:update", HGResource{ScopeType: "region", ScopeID: "unknown"})
	if !errors.Is(err, ErrHGScopeNotFound) || decision.Allowed {
		t.Fatalf("未登记资源不能授权: decision=%+v err=%v", decision, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 已登记的新作用域在业务服务提供服务端可信 scope ID 时，可按精确绑定权限授权。
func TestHGAuthorizeRegisteredScope(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeResourceActive)).
		WithArgs("content_channel", "channel-1").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "content_channel", "channel-1", "video:submission:read", true)
	decision, err := service.Authorize(context.Background(), "ACTOR", "video:submission:read",
		HGResource{ScopeType: "content_channel", ScopeID: "channel-1"})
	if err != nil || !decision.Allowed {
		t.Fatalf("已登记资源的精确授权应通过: decision=%+v err=%v", decision, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 六类资源在同一事务写入 IAM 登记与真实业务实体，任一失败均不能留下可授权的孤儿资源。
func TestHGCreateBusinessScopeAtomic(t *testing.T) {
	for _, tc := range []struct {
		name, scopeType, owner, code, organization, insert string
		args                                               []driver.Value
	}{
		{"创作者", "creator", "ACTOR", "", "", SQLQueriesPackage.HGInsertCreatorSpace, []driver.Value{sqlmock.AnyArg(), "ACTOR", "新资源", "ACTOR"}},
		{"组织", "organization", "", "", "", SQLQueriesPackage.HGInsertBusinessOrganization, []driver.Value{sqlmock.AnyArg(), "新资源", "ACTOR"}},
		{"环境", "environment", "", "staging", "", SQLQueriesPackage.HGInsertOperationEnvironment, []driver.Value{sqlmock.AnyArg(), "staging", "新资源", "ACTOR"}},
		{"爬虫项目", "crawler_project", "", "", "ORG", SQLQueriesPackage.HGInsertCrawlerProject, []driver.Value{sqlmock.AnyArg(), "ORG", "新资源", "ACTOR"}},
		{"频道", "content_channel", "", "", "", SQLQueriesPackage.HGInsertContentChannel, []driver.Value{sqlmock.AnyArg(), nil, "新资源", "ACTOR"}},
		{"区域", "region", "", "north", "", SQLQueriesPackage.HGInsertBusinessRegion, []driver.Value{sqlmock.AnyArg(), "north", "新资源", "ACTOR"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, mock := hgTestService(t)
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).
				WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
			hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
			hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeResourceRequest)).
				WithArgs("ACTOR", "request-1").WillReturnRows(sqlmock.NewRows([]string{"scope_id", "scope_type", "scope_name"}))
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMActiveScopeType)).
				WithArgs(tc.scopeType).WillReturnRows(sqlmock.NewRows([]string{"requires_scope_id"}).AddRow(true))
			if tc.owner != "" {
				mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantTargetUser)).
					WithArgs(tc.owner).WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
			}
			if tc.organization != "" {
				mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectBusinessOrganization)).
					WithArgs(tc.organization).WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
			}
			mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMBusinessScopeResource)).
				WithArgs(tc.scopeType, sqlmock.AnyArg(), "新资源", "ACTOR", "request-1").
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec(regexp.QuoteMeta(tc.insert)).WithArgs(tc.args...).
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).
				WithArgs("ACTOR", "iam.business_scope.create", tc.scopeType, sqlmock.AnyArg(), "request-1", sqlmock.AnyArg()).
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectCommit()
			item, err := service.HGCreateBusinessScope(context.Background(), "ACTOR", "request-1", HGBusinessScopeCreate{
				ScopeType: tc.scopeType, Name: "新资源", OwnerUserID: tc.owner, Code: tc.code, OrganizationID: tc.organization,
			})
			if err != nil || len(item.ID) != 32 || item.Name != "新资源" {
				t.Fatalf("权威业务资源创建失败: item=%+v err=%v", item, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGCreateBusinessScopeRejectsAnotherCreatorOwner(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).
		WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	if _, err := service.HGCreateBusinessScope(context.Background(), "ACTOR", "request-1", HGBusinessScopeCreate{
		ScopeType: "creator", Name: "别人的创作者空间", OwnerUserID: "OTHER",
	}); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("管理员不能冒充其他用户建创作者空间: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGCreateBusinessScopeRollbackOnBusinessError(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).
		WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeResourceRequest)).
		WithArgs("ACTOR", "request-1").WillReturnRows(sqlmock.NewRows([]string{"scope_id", "scope_type", "scope_name"}))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMActiveScopeType)).
		WithArgs("region").WillReturnRows(sqlmock.NewRows([]string{"requires_scope_id"}).AddRow(true))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMBusinessScopeResource)).
		WithArgs("region", sqlmock.AnyArg(), "北方", "ACTOR", "request-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertBusinessRegion)).
		WithArgs(sqlmock.AnyArg(), "north", "北方", "ACTOR").WillReturnError(errors.New("数据库写入失败"))
	mock.ExpectRollback()
	if _, err := service.HGCreateBusinessScope(context.Background(), "ACTOR", "request-1", HGBusinessScopeCreate{
		ScopeType: "region", Code: "north", Name: "北方",
	}); err == nil {
		t.Fatal("业务实体失败时 IAM 登记必须回滚")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 同 requestID 只有匹配的实体才可复用；历史 IAM-only 记录不能被重放成真实实体。
func TestHGCreateBusinessScopeReplayRejectsPlaceholder(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).
		WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeResourceRequest)).
		WithArgs("ACTOR", "request-1").WillReturnRows(sqlmock.NewRows([]string{"scope_id", "scope_type", "scope_name"}).
		AddRow("placeholder-id", "region", "北方"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectBusinessRegion)).
		WithArgs("placeholder-id").WillReturnRows(sqlmock.NewRows([]string{"owner", "code", "organization_id"}))
	if _, err := service.HGCreateBusinessScope(context.Background(), "ACTOR", "request-1", HGBusinessScopeCreate{
		ScopeType: "region", Code: "north", Name: "北方",
	}); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("占位 IAM 记录不得冒充业务实体: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 停用同时改变业务状态、IAM 候选与策略版本；历史绑定仍可审计但不能授权。
func TestHGDisableBusinessScopeAtomic(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).
		WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeTypeForUpdate)).
		WithArgs("region").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMBusinessScopeForUpdate)).
		WithArgs("region", "region-id").WillReturnRows(sqlmock.NewRows([]string{"status", "business_verified"}).AddRow(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGDisableBusinessRegion)).
		WithArgs("region-id").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGDisableIAMBusinessScope)).
		WithArgs("region", "region-id").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGBumpIAMPolicyRevision)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).
		WithArgs("ACTOR", "iam.business_scope.disable", "region", "region-id", "request-2", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := service.HGDisableBusinessScope(context.Background(), "ACTOR", "request-2", "region", "region-id"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGDisableOrganizationRejectsActiveChildren(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).
		WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeTypeForUpdate)).
		WithArgs("organization").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMBusinessScopeForUpdate)).
		WithArgs("organization", "org-id").WillReturnRows(sqlmock.NewRows([]string{"status", "business_verified"}).AddRow(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectActiveProjectByOrganization)).
		WithArgs("org-id").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	mock.ExpectRollback()
	if err := service.HGDisableBusinessScope(context.Background(), "ACTOR", "request-2", "organization", "org-id"); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("有活动子项目不能停用组织: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGBusinessScopeRejectsInvalidMetadata(t *testing.T) {
	for _, req := range []HGBusinessScopeCreate{
		{ScopeType: "creator", Name: "空间"},
		{ScopeType: "creator", Name: "空间", OwnerUserID: "OWNER", OrganizationID: "ORG"},
		{ScopeType: "region", Name: "北方", Code: "north;DROP"},
		{ScopeType: "environment", Name: "环境"},
		{ScopeType: "organization", Name: "组织", OwnerUserID: "OTHER"},
		{ScopeType: "crawler_project", Name: "项目", Code: "staging"},
		{ScopeType: "content_channel", Name: "频道", OrganizationID: strings.Repeat("x", 65)},
	} {
		if hgValidBusinessScopeRequest(req) {
			t.Fatalf("元数据不合法却被接受: %+v", req)
		}
	}
}

// IAM 候选、授予与业务 Authorize 均必须复核权威表；历史占位资源绝不可单靠 IAM 登记生效。
func TestHGBusinessScopeQueriesRequireAuthority(t *testing.T) {
	for _, query := range []string{
		SQLQueriesPackage.HGSelectIAMScopeResourceActive,
		SQLQueriesPackage.HGSelectIAMScopeResourcesFirst,
		SQLQueriesPackage.HGSelectIAMScopeResourcesAfter,
	} {
		if !strings.Contains(query, "r.business_verified=1") {
			t.Error("不能对历史占位记录执行深扫描或赋予业务权限")
		}
		for _, table := range []string{
			"hg_creator_spaces", "hg_business_organizations", "hg_operation_environments",
			"hg_crawler_projects", "hg_content_channels", "hg_business_regions",
		} {
			if !strings.Contains(query, "FROM "+table+" ") || !strings.Contains(query, "b.status=1") {
				t.Errorf("授权/候选查询未复核 %s 的有效状态", table)
			}
		}
	}
}

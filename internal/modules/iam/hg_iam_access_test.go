package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGAuthorizeProductUpdateUsesTrustedState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewHGService(NewHGRepository(db))
	columns := []string{"user_id", "merchant_id", "merchant_status", "product_status", "review_status"}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallProductAuthStateSQL)).WithArgs("P1").
		WillReturnRows(sqlmock.NewRows(columns).AddRow("OTHER", "M1", 1, 1, 1))
	decision, err := service.HGAuthorizeProductUpdate(context.Background(), "U1", "P1")
	if err != nil || decision.Allowed {
		t.Fatalf("他人商品不应放行: %+v %v", decision, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallProductAuthStateSQL)).WithArgs("P1").
		WillReturnRows(sqlmock.NewRows(columns).AddRow("U1", "M1", 1, 0, 1))
	decision, err = service.HGAuthorizeProductUpdate(context.Background(), "U1", "P1")
	if err != nil || decision.Allowed || decision.Reason != "资源状态不允许操作" {
		t.Fatalf("非已上架商品不能判为可编辑: %+v %v", decision, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallProductAuthStateSQL)).WithArgs("P1").
		WillReturnRows(sqlmock.NewRows(columns).AddRow("U1", "M1", 1, 1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMPermission)).
		WithArgs("U1", "global", "", "mall:product:update").
		WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	decision, err = service.HGAuthorizeProductUpdate(context.Background(), "U1", "P1")
	if err != nil || !decision.Allowed {
		t.Fatalf("本人已上架商品和有效全局权限应通过预判: %+v %v", decision, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallProductAuthStateSQL)).WithArgs("P1").
		WillReturnError(errors.New("数据库不可用"))
	decision, err = service.HGAuthorizeProductUpdate(context.Background(), "U1", "P1")
	if err == nil || decision.Allowed {
		t.Fatalf("数据源故障不能伪装成正常拒绝: %+v %v", decision, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGAccessSnapshotKeepsScopeBindings(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC().Add(30 * time.Second)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMPolicyRevision)).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(19))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMAccessGrants)).WithArgs("U1").
		WillReturnRows(sqlmock.NewRows([]string{"role_code", "permission_code", "scope_type", "scope_id", "expires_at"}).
			AddRow("merchant", "mall:product:update", "shop", "A", now).
			AddRow("merchant", "mall:product:publish", "shop", "A", now).
			AddRow("operator", "ops:metrics:read", "global", "", nil))
	snapshot, err := NewHGService(NewHGRepository(db)).AccessSnapshot(context.Background(), "U1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.PolicyVersion != 19 || len(snapshot.Roles) != 2 || len(snapshot.Grants) != 3 ||
		snapshot.Grants[0].ScopeID != "A" || snapshot.Grants[2].ScopeType != "global" ||
		snapshot.ExpiresAt.After(now) {
		t.Fatalf("授权投影范围或有效期不正确: %+v", snapshot)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGAccessSnapshotRejectsOverflow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := sqlmock.NewRows([]string{"role_code", "permission_code", "scope_type", "scope_id", "expires_at"})
	for i := 0; i < 1001; i++ {
		rows.AddRow("role", "iam:role:read", "global", "", nil)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMPolicyRevision)).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMAccessGrants)).WithArgs("U1").WillReturnRows(rows)
	_, err = NewHGService(NewHGRepository(db)).AccessSnapshot(context.Background(), "U1")
	if err == nil {
		t.Fatal("超出上限不能返回截断授权")
	}
}

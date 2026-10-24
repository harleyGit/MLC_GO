package product

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

type hgReviewAuthorizer struct{ allowed bool } // 服务端权限查询结果；false 表示无审核权限。
func (a hgReviewAuthorizer) HasAssetPermission(_ context.Context, _, _ string) (bool, error) {
	return a.allowed, nil
}

type hgReviewStore struct {
	HGStore     // 未调用的商家和商品方法由嵌入接口占位，测试只断言审核权限边界。
	calls   int // 商品审核写入次数，无权限时必须为0。
}

func (s *hgReviewStore) ReviewProduct(_ context.Context, _, _ string, _ int8) error {
	s.calls++
	return nil
}

func TestHGReviewProductChecksDatabaseAuthorizationBeforeWrite(t *testing.T) {
	store := &hgReviewStore{}
	service := HGNewService(store, hgReviewAuthorizer{allowed: false})
	if err := service.ReviewProduct(context.Background(), "operator", "PRD_1", true); !errors.Is(err, ErrHGForbidden) || store.calls != 0 {
		t.Fatalf("无审核权限仍访问仓储: calls=%d err=%v", store.calls, err)
	}
	service = HGNewService(store, hgReviewAuthorizer{allowed: true})
	if err := service.ReviewProduct(context.Background(), "operator", "PRD_1", true); err != nil || store.calls != 1 {
		t.Fatalf("授权商品审核失败: calls=%d err=%v", store.calls, err)
	}
}

func TestHGRejectProductKeepsLifecycleDisabled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallProductReviewSQL)).WithArgs(int8(0), int8(2), "PRD_1", "operator").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := HGNewRepository(db).ReviewProduct(context.Background(), "operator", "PRD_1", 2); err != nil {
		t.Fatalf("商品拒绝审核失败: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGPublishRejectsUnsellablePhysicalPriceAndMissingStock(t *testing.T) {
	service := HGNewService(nil, nil)
	draft := HGProductDraft{Type: "PHYSICAL_PRODUCT", Category: "digital", Title: "实物", Summary: "正品", SKUTitle: "标准", Amount: 100001, Stock: 1}
	if _, err := service.Publish(context.Background(), "seller", draft); !errors.Is(err, ErrHGInvalid) {
		t.Fatalf("超过可支付上限的实物不得发布: %v", err)
	}
	draft.Amount, draft.Stock = 100, 0
	if _, err := service.Publish(context.Background(), "seller", draft); !errors.Is(err, ErrHGInvalid) {
		t.Fatalf("无库存实物不得发布: %v", err)
	}
}

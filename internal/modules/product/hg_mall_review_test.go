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
	HGStore                         // 未调用的商家和商品方法由嵌入接口占位，测试只断言审核权限边界。
	calls   int                     // 商品审核写入次数，无权限时必须为0。
	pending []HGMerchantApplication // 模拟待审商家原始游标页，包含可能属于审核者自己的申请。
}

func (s *hgReviewStore) ReviewProduct(_ context.Context, _, _ string, _ int8) error {
	s.calls++
	return nil
}

func (s *hgReviewStore) PendingMerchants(_ context.Context, _ uint64) ([]HGMerchantApplication, error) {
	return s.pending, nil
}

func TestHGPendingMerchantsExcludesReviewersOwnApplicationWithoutSkippingCursor(t *testing.T) {
	store := &hgReviewStore{}
	for id := uint64(21); id > 0; id-- {
		owner := "another-user"
		if id == 20 {
			owner = "operator"
		}
		store.pending = append(store.pending, HGMerchantApplication{ID: id, MerchantID: "MER_1", UserID: owner})
	}
	page, err := HGNewService(store, hgReviewAuthorizer{allowed: true}).PendingMerchants(context.Background(), "operator", "")
	if err != nil || len(page.List) != 19 || !page.HasMore || page.NextCursor != "2" {
		t.Fatalf("应隐藏本人申请且保留原始游标：page=%+v err=%v", page, err)
	}
	for _, item := range page.List {
		if item.UserID == "operator" {
			t.Fatal("本人申请不得进入可审核列表")
		}
	}
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

func TestHGReviewMerchantKeepsConditionalUpdateOnConcurrentChange(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, count := range []int64{1, 0} {
		mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallMerchantApproveSQL)).
			WithArgs(int8(1), "operator", "MER_1", "operator").WillReturnResult(sqlmock.NewResult(0, count))
	}
	repo := HGNewRepository(db)
	if err := repo.ReviewMerchant(context.Background(), "operator", "MER_1", 1); err != nil {
		t.Fatalf("其他用户的待审商家应审核成功: %v", err)
	}
	if err := repo.ReviewMerchant(context.Background(), "operator", "MER_1", 1); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("状态变化或本人申请不得绕过条件更新: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
	draft.Stock, draft.ImageURLs = 1, []string{"one", "two", "three", "four", "five", "six"}
	if _, err := service.Publish(context.Background(), "seller", draft); !errors.Is(err, ErrHGInvalid) {
		t.Fatalf("超过图片上限不得发布: %v", err)
	}
}

func TestHGPublishImageOwnershipIsAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallMerchantApprovedForUpdateSQL)).WithArgs("seller").WillReturnRows(sqlmock.NewRows([]string{"merchant_id"}).AddRow("MER_1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallProductInsertSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallImageBindSQL)).WithArgs("PRD_1", "https://cdn.example/image.jpg", "https://cdn.example/image.jpg", "seller").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	item := HGCatalogItem{ProductID: "PRD_1", MerchantID: "MER_1", ImageURLs: []string{"https://cdn.example/image.jpg"}}
	if err := HGNewRepository(db).Publish(context.Background(), "seller", item); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("非本人或已绑定图片应回滚: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGDetailReturnsProductImagesAndIntroduction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallCatalogDetailSQL)).WithArgs("PRD_1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "product_type", "title", "category", "summary", "merchant_id", "shop_name", "sku_id", "sku_title", "amount", "available", "description", "image_urls"}).
			AddRow(1, "PRD_1", "PHYSICAL_PRODUCT", "商品", "digital", "摘要", "MER_1", "店铺", "SKU_1", "标准", 100, 2, "商品展示介绍", `["/uploads/mall_product/a.png","/uploads/mall_product/b.png"]`))
	item, err := HGNewRepository(db).Detail(context.Background(), "PRD_1")
	if err != nil || item.Description != "商品展示介绍" || len(item.ImageURLs) != 2 || item.ImageURLs[1] != "/uploads/mall_product/b.png" {
		t.Fatalf("商品介绍与多图读取失败: %+v %v", item, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

package product

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGStageMediaBindsOnlyOwnersUnusedImages(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	url := "/uploads/mall_product/a.png"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallMediaOwnProductSQL)).WithArgs("PRD_1", "seller").WillReturnRows(sqlmock.NewRows([]string{"image_urls"}).AddRow(`[]`))
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallMediaPendingCountSQL)).WithArgs("PRD_1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallMediaStageSQL)).WithArgs("PRD_1", url, url, "seller").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	if err := HGNewRepository(db).StageMedia(context.Background(), "seller", "PRD_1", []string{url}); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("非本人上传的图片不应绑定: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGStageMediaQueuesExistingPublishedProductWithoutChangingVisibility(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	url := "/uploads/mall_product/new.png"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallMediaOwnProductSQL)).WithArgs("PRD_1", "seller").WillReturnRows(sqlmock.NewRows([]string{"image_urls"}).AddRow(`["/uploads/mall_product/old.png"]`))
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallMediaPendingCountSQL)).WithArgs("PRD_1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallMediaStageSQL)).WithArgs("PRD_1", url, url, "seller").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := HGNewRepository(db).StageMedia(context.Background(), "seller", "PRD_1", []string{url}); err != nil {
		t.Fatalf("已有商品补图排队失败: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGReviewMediaPublishesApprovedImageWithoutUnlistingProduct(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	url := "/uploads/mall_product/a.png"
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallMediaTargetSQL)).WithArgs(url, url).WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow("PRD_1"))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallMediaReviewerProductSQL)).WithArgs("PRD_1", "reviewer").WillReturnRows(sqlmock.NewRows([]string{"image_urls"}).AddRow(`[]`))
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallMediaApproveImageSQL)).WithArgs(url, url, "PRD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallMediaPublishSQL)).WithArgs([]byte(`["/uploads/mall_product/a.png"]`), "PRD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := HGNewRepository(db).ReviewMedia(context.Background(), "reviewer", url, true); err != nil {
		t.Fatalf("审核通过后补图应公开: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGReviewMediaRejectsSelfReview(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	url := "/uploads/mall_product/a.png"
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallMediaTargetSQL)).WithArgs(url, url).WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow("PRD_1"))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallMediaReviewerProductSQL)).WithArgs("PRD_1", "seller").WillReturnRows(sqlmock.NewRows([]string{"image_urls"}))
	mock.ExpectRollback()
	if err := HGNewRepository(db).ReviewMedia(context.Background(), "seller", url, true); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("本人不能审核自己的补图: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGReviewMediaRequiresReviewerPermission(t *testing.T) {
	store := &hgReviewStore{}
	service := HGNewService(store, hgReviewAuthorizer{allowed: false})
	if err := service.HGReviewMedia(context.Background(), "ordinary-user", "/uploads/mall_product/a.png", true); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("普通用户不得审核补图: %v", err)
	}
}

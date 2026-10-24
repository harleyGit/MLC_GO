package product

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGMineDetailRequiresOwnershipAndReturnsArchivedStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	columns := []string{"id", "product_id", "product_type", "title", "category", "summary", "merchant_id", "shop_name", "sku_id", "sku_title", "amount", "available", "review_status", "status", "description", "image_urls"}
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallMineByProductIDSQL)).WithArgs("PRD_1", "seller").WillReturnRows(sqlmock.NewRows(columns).AddRow(1, "PRD_1", "PHYSICAL_PRODUCT", "商品", "digital", "摘要", "MER_1", "店铺", "SKU_1", "规格", 100, 0, 1, 2, "介绍", `["/uploads/a.jpg"]`))
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallMineByProductIDSQL)).WithArgs("PRD_1", "other").WillReturnRows(sqlmock.NewRows(columns))
	repo := HGNewRepository(db)
	item, err := repo.MineDetail(context.Background(), "seller", "PRD_1")
	if err != nil || item.LifecycleStatus != 2 || len(item.ImageURLs) != 1 {
		t.Fatalf("本人已归档商品查询失败: %+v %v", item, err)
	}
	if _, err = repo.MineDetail(context.Background(), "other", "PRD_1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("他人商品不应可查: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGArchiveOnlyUpdatesOwnerProduct(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallMineArchiveSQL)).WithArgs("PRD_1", "other").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	if err := HGNewRepository(db).Archive(context.Background(), "other", "PRD_1"); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("非本人不能归档: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGArchiveCancelsPendingChangesInSameTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallMineArchiveSQL)).WithArgs("PRD_1", "seller").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallMineCancelMediaSQL)).WithArgs("PRD_1").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallMineCancelEditSQL)).WithArgs("PRD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := HGNewRepository(db).Archive(context.Background(), "seller", "PRD_1"); err != nil {
		t.Fatalf("本人商品应归档并撤销待审修改: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGStageEditLeavesPublishedProductUntouched(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	req := HGEditRequest{ProductID: "PRD_1", Title: "新标题", Summary: "新简介", Description: "新介绍"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallEditOwnSQL)).WithArgs("PRD_1", "seller").WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow("PRD_1"))
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallEditStatusSQL)).WithArgs("PRD_1").WillReturnRows(sqlmock.NewRows([]string{"status"}))
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallEditInsertSQL)).WithArgs("PRD_1", "seller", "新标题", "新简介", "新介绍").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := HGNewRepository(db).StageEdit(context.Background(), "seller", req); err != nil {
		t.Fatalf("本人编辑申请应进入审核队列: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGReviewEditPublishesApprovedTextAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallEditReviewerProductSQL)).WithArgs("PRD_1", "reviewer").WillReturnRows(sqlmock.NewRows([]string{"product_id"}).AddRow("PRD_1"))
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallEditPendingOneSQL)).WithArgs("PRD_1").WillReturnRows(sqlmock.NewRows([]string{"title", "summary", "description"}).AddRow("新标题", "新简介", "新介绍"))
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallEditPublishSQL)).WithArgs("新标题", "新简介", "新介绍", "PRD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(q.HGMallEditDecisionSQL)).WithArgs(int8(1), "reviewer", "PRD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := HGNewRepository(db).ReviewEdit(context.Background(), "reviewer", "PRD_1", true); err != nil {
		t.Fatalf("编辑审核应原子发布: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGReviewEditRejectsOwnerAsReviewer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGMallEditReviewerProductSQL)).WithArgs("PRD_1", "seller").WillReturnRows(sqlmock.NewRows([]string{"product_id"}))
	mock.ExpectRollback()
	if err := HGNewRepository(db).ReviewEdit(context.Background(), "seller", "PRD_1", true); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("本人编辑不得自审: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

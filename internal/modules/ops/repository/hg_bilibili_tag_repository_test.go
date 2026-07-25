package OpsRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreateBilibiliTagUsesBusinessTagID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOpsBilibiliTagSQL)).
		WithArgs(sqlmock.AnyArg(), "MMD·3D", 20, 1, "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P", "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P").
		WillReturnResult(sqlmock.NewResult(101, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsBilibiliTagByTagIDSQL)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"tag_id", "id", "name", "sort_order", "status", "created_at", "updated_at"}).
			AddRow("BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P", int64(101), "MMD·3D", 20, 1, time.Now(), time.Now()))

	repo := NewRepository(db)
	item, err := repo.CreateBilibiliTag(context.Background(), "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P", "MMD·3D", 20, 1)
	if err != nil {
		t.Fatalf("CreateBilibiliTag() error = %v", err)
	}
	if item["tagId"] != "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P" {
		t.Fatalf("tagId = %v, want business tag id", item["tagId"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUpdateBilibiliTagUsesBusinessTagID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	tagID := "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P"
	operatorID := "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P"
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateOpsBilibiliTagSQL)).
		WithArgs("MMD·3D", 30, 2, operatorID, tagID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsBilibiliTagByTagIDSQL)).
		WithArgs(tagID).
		WillReturnRows(sqlmock.NewRows([]string{"tag_id", "id", "name", "sort_order", "status", "created_at", "updated_at"}).
			AddRow(tagID, int64(101), "MMD·3D", 30, 2, time.Now(), time.Now()))

	repo := NewRepository(db)
	item, err := repo.UpdateBilibiliTag(context.Background(), operatorID, tagID, "MMD·3D", 30, 2)
	if err != nil {
		t.Fatalf("UpdateBilibiliTag() error = %v", err)
	}
	if item["tagId"] != tagID {
		t.Fatalf("tagId = %v, want %s", item["tagId"], tagID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestDeleteBilibiliTagUsesBusinessTagID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	tagID := "BLTAG_01K10D6JQS9XV3GR2F7B5M8N4P"
	operatorID := "HGUSR_TW_01K10D6JQS9XV3GR2F7B5M8N4P"
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DeleteOpsBilibiliTagSQL)).
		WithArgs(operatorID, tagID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewRepository(db)
	if err := repo.DeleteBilibiliTag(context.Background(), operatorID, tagID); err != nil {
		t.Fatalf("DeleteBilibiliTag() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

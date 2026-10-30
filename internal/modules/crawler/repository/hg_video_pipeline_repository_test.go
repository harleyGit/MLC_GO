package repository

import (
	CrawlerEventPackage "MLC_GO/internal/events/crawler"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
)

func TestHGAdvanceStageOutboxFailureRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	item := CrawlerEventPackage.HGVideoEvent{BVID: "BV1GJ411x7h7", CollectionID: "f3e1d50a-b910-4f55-a1e6-6bbfc92f6f39"}
	event, _ := CrawlerEventPackage.HGEnvelope(CrawlerEventPackage.HGRaw, item)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGLockVideoCollectionSQL)).WithArgs(item.CollectionID).WillReturnRows(sqlmock.NewRows([]string{"bvid", "status"}).AddRow(item.BVID, "pending"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOutboxEventSQL)).WillReturnError(errors.New("模拟Outbox写失败"))
	mock.ExpectRollback()
	if err := NewRepository(db).HGAdvanceVideo(context.Background(), item, "pending", "raw", "", &event, nil); err == nil {
		t.Fatal("Outbox失败不能推进阶段")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGAdvanceDuplicateDoesNotRepublish(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	item := CrawlerEventPackage.HGVideoEvent{BVID: "BV1GJ411x7h7", CollectionID: "f3e1d50a-b910-4f55-a1e6-6bbfc92f6f39"}
	event, _ := CrawlerEventPackage.HGEnvelope(CrawlerEventPackage.HGRaw, item)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGLockVideoCollectionSQL)).WithArgs(item.CollectionID).WillReturnRows(sqlmock.NewRows([]string{"bvid", "status"}).AddRow(item.BVID, "raw"))
	mock.ExpectRollback()
	if err := NewRepository(db).HGAdvanceVideo(context.Background(), item, "pending", "raw", "", &event, nil); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestHGBackfillRollbackKeepsCursor 验证补采事件失败整页回滚，并保留原游标供恢复。
func TestHGBackfillRollbackKeepsCursor(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgMock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGBackfillVideoPageSQL)).WithArgs(uint64(10)).WillReturnRows(sqlmock.NewRows([]string{"id", "platform", "content_id", "skip"}).AddRow(11, "bilibili", "BV1GJ411x7h7", false))
	hgMock.ExpectBegin()
	hgMock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertVideoCollectionSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	hgMock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOutboxEventSQL)).WillReturnError(errors.New("模拟Outbox失败"))
	hgMock.ExpectRollback()
	hgNext, hgCount, hgErr := NewRepository(hgDB).WithHGVideoPipeline(true).HGBackfillVideoPage(context.Background(), 10)
	if hgErr == nil || hgNext != 10 || hgCount != 0 {
		t.Fatalf("失败页不应推进游标: %d %d %v", hgNext, hgCount, hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

// TestHGBackfillTaskSkipsStoredAndInFlight 确认只扫描指定任务，已入库/在途不会再建采集事件。
func TestHGBackfillTaskSkipsStoredAndInFlight(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgMock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGBackfillTaskVideoPageSQL)).
		WithArgs(uint64(1), uint64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "platform", "content_id", "skip"}).
			AddRow(11, "bilibili", "BV1GJ411x7h7", true).
			AddRow(12, "bilibili", "BV1ow8p6NEDP", true).
			AddRow(13, "other", "unrelated", false))
	hgNext, hgCount, hgErr := NewRepository(hgDB).WithHGVideoPipeline(true).HGBackfillTaskVideoPage(context.Background(), 1, 10)
	if hgErr != nil || hgNext != 13 || hgCount != 0 {
		t.Fatalf("已采集/在途或非B站应跳过: next=%d count=%d err=%v", hgNext, hgCount, hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

// TestHGBackfillTaskCreatesOnlyMissingEvent 确认缺失视频只提交一次发现事件并推进关联游标。
func TestHGBackfillTaskCreatesOnlyMissingEvent(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgMock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGBackfillTaskVideoPageSQL)).
		WithArgs(uint64(1), uint64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "platform", "content_id", "skip"}).
			AddRow(5, "bilibili", "BV1GJ411x7h7", false).
			AddRow(6, "bilibili", "BV1ow8p6NEDP", true))
	hgMock.ExpectBegin()
	hgMock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertVideoCollectionSQL)).
		WithArgs(sqlmock.AnyArg(), "BV1GJ411x7h7", uint64(0), uint64(0), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	hgMock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOutboxEventSQL)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	hgMock.ExpectCommit()
	hgNext, hgCount, hgErr := NewRepository(hgDB).WithHGVideoPipeline(true).HGBackfillTaskVideoPage(context.Background(), 1, 0)
	if hgErr != nil || hgNext != 6 || hgCount != 1 {
		t.Fatalf("只应补发缺失视频: next=%d count=%d err=%v", hgNext, hgCount, hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

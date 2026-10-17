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

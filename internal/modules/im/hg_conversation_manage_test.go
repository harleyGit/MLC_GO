package im

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGDeleteConversationRejectsNonMember(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(SQL.HGIMLockConversationSQL).WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"low", "high", "seq"}).AddRow("a", "b", 8))
	mock.ExpectRollback()
	_, _, err = HGNewService(db, "", nil).HGDeleteConversationBatch(context.Background(), "other", 7, 0)
	if !errors.Is(err, HGErrForbidden) {
		t.Fatalf("非会话成员应被拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGDeleteConversationKeepsRecentAndHidesOnlyOwner(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(SQL.HGIMLockConversationSQL).WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"low", "high", "seq"}).AddRow("a", "b", 8))
	mock.ExpectQuery(SQL.HGIMDeleteProgressSQL).WithArgs(uint64(7), "a").WillReturnRows(sqlmock.NewRows([]string{"deleted_seq", "deleted_at"}).AddRow(0, nil))
	mock.ExpectExec(SQL.HGIMStageConversationDeleteSQL).WithArgs(uint64(8), uint64(7), "a").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(SQL.HGIMOldMessagesSQL).WithArgs(uint64(7), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(101))
	for _, query := range []string{SQL.HGIMDeleteJobSQL, SQL.HGIMDeleteDeliverySQL, SQL.HGIMDeleteAttachmentLinkSQL} {
		mock.ExpectExec(query).WithArgs(uint64(101)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec(SQL.HGIMDeleteMessageSQL).WithArgs(uint64(101), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(SQL.HGIMHideConversationSQL).WithArgs(uint64(8), uint64(7), "a").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	seq, more, err := HGNewService(db, "", nil).HGDeleteConversationBatch(context.Background(), "a", 7, 0)
	if err != nil || more || seq != 8 {
		t.Fatalf("最后一批应完成隐藏: seq=%d more=%v err=%v", seq, more, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGDeleteConversationRejectsChangedBase(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(SQL.HGIMLockConversationSQL).WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"low", "high", "seq"}).AddRow("a", "b", 10))
	mock.ExpectQuery(SQL.HGIMDeleteProgressSQL).WithArgs(uint64(7), "a").WillReturnRows(sqlmock.NewRows([]string{"deleted_seq", "deleted_at"}).AddRow(8, nil))
	mock.ExpectRollback()
	_, _, err = HGNewService(db, "", nil).HGDeleteConversationBatch(context.Background(), "a", 7, 9)
	if !errors.Is(err, HGErrInvalid) {
		t.Fatalf("伪造水位应拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGDeleteConversationAgainCapturesNewMessages(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(SQL.HGIMLockConversationSQL).WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"low", "high", "seq"}).AddRow("a", "b", 12))
	mock.ExpectQuery(SQL.HGIMDeleteProgressSQL).WithArgs(uint64(7), "a").WillReturnRows(sqlmock.NewRows([]string{"deleted_seq", "deleted_at"}).AddRow(8, time.Now()))
	mock.ExpectExec(SQL.HGIMStageConversationDeleteSQL).WithArgs(uint64(12), uint64(7), "a").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(SQL.HGIMOldMessagesSQL).WithArgs(uint64(7), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(SQL.HGIMHideConversationSQL).WithArgs(uint64(12), uint64(7), "a").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	seq, more, err := HGNewService(db, "", nil).HGDeleteConversationBatch(context.Background(), "a", 7, 0)
	if err != nil || more || seq != 12 {
		t.Fatalf("再次删除应以最新 seq 隐藏: %d %v %v", seq, more, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGDeleteConversationLimitsOneBatch(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(SQL.HGIMLockConversationSQL).WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"low", "high", "seq"}).AddRow("a", "b", 21))
	mock.ExpectQuery(SQL.HGIMDeleteProgressSQL).WithArgs(uint64(7), "a").WillReturnRows(sqlmock.NewRows([]string{"deleted_seq", "deleted_at"}).AddRow(21, nil))
	rows := sqlmock.NewRows([]string{"id"})
	for id := 1; id <= 21; id++ {
		rows.AddRow(id)
	}
	mock.ExpectQuery(SQL.HGIMOldMessagesSQL).WithArgs(uint64(7), sqlmock.AnyArg()).WillReturnRows(rows)
	for id := 1; id <= 20; id++ {
		for _, query := range []string{SQL.HGIMDeleteJobSQL, SQL.HGIMDeleteDeliverySQL, SQL.HGIMDeleteAttachmentLinkSQL} {
			mock.ExpectExec(query).WithArgs(uint64(id)).WillReturnResult(sqlmock.NewResult(0, 1))
		}
		mock.ExpectExec(SQL.HGIMDeleteMessageSQL).WithArgs(uint64(id), uint64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()
	seq, more, err := HGNewService(db, "", nil).HGDeleteConversationBatch(context.Background(), "a", 7, 21)
	if err != nil || !more || seq != 21 {
		t.Fatal(fmt.Sprintf("不能在未完成批次隐藏会话: seq=%d more=%v err=%v", seq, more, err))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGSetConversationMutedRejectsMissingMember(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec(SQL.HGIMSetMutedSQL).WithArgs(true, uint64(7), "other").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(SQL.HGIMConversationMemberSQL).WithArgs(uint64(7), "other").WillReturnError(sql.ErrNoRows)
	if err := HGNewService(db, "", nil).HGSetConversationMuted(context.Background(), "other", 7, true); !errors.Is(err, HGErrForbidden) {
		t.Fatalf("非本人设置应拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGConversationByIDUsesServerSequence(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hgTime := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(SQL.HGIMConversationByIDSQL).WithArgs(uint64(7), "a").WillReturnRows(
		sqlmock.NewRows([]string{"last_seq", "last_read_seq", "updated_at", "low", "high", "muted"}).
			AddRow(42, 30, hgTime, "a", "b", true))
	item, err := HGNewService(db, "", nil).HGConversationByID(context.Background(), "a", 7)
	if err != nil || item.ConversationID != "7" || item.Peer.UserID != "b" || item.LastSeq != 42 || !item.Muted {
		t.Fatalf("会话快照应包含历史序号和本人设置: %+v, %v", item, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

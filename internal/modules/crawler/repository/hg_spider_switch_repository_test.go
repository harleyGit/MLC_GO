package repository

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGSpiderSwitchPersistsTransitionAndAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGLockSpiderSwitchSQL)).WillReturnRows(sqlmock.NewRows([]string{"desired_running"}).AddRow(false))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGUpdateSpiderSwitchSQL)).WithArgs(true, "admin").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertSpiderSwitchEventSQL)).WithArgs(true, "admin").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	changed, err := NewHGSpiderSwitchRepository(db).HGSetDesiredRunning(context.Background(), true, "admin")
	if err != nil || !changed {
		t.Fatalf("切换与审计必须一同提交: changed=%t err=%v", changed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGSpiderSwitchIdempotentNoExtraAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGLockSpiderSwitchSQL)).WillReturnRows(sqlmock.NewRows([]string{"desired_running"}).AddRow(true))
	mock.ExpectCommit()
	changed, err := NewHGSpiderSwitchRepository(db).HGSetDesiredRunning(context.Background(), true, "admin")
	if err != nil || changed {
		t.Fatalf("重复启动不得生成切换记录: changed=%t err=%v", changed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGSpiderSwitchAuditFailureRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGLockSpiderSwitchSQL)).WillReturnRows(sqlmock.NewRows([]string{"desired_running"}).AddRow(false))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGUpdateSpiderSwitchSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertSpiderSwitchEventSQL)).WillReturnError(errors.New("审计写入失败"))
	mock.ExpectRollback()
	changed, err := NewHGSpiderSwitchRepository(db).HGSetDesiredRunning(context.Background(), true, "admin")
	if err == nil || changed {
		t.Fatalf("审计失败不能改变权威状态: changed=%t err=%v", changed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

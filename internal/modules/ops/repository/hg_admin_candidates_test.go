package OpsRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestHGListAvailableAdminCandidates 验证首末页与游标页均只返回有界候选，游标取实际返回行的最后一个 ID。
func TestHGListAvailableAdminCandidates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewRepository(db)
	cols := []string{"id", "user_id", "user_name", "nickname", "email", "phone"}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAvailableAdminCandidateWindowSQL)).WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11).AddRow(9).AddRow(7)).RowsWillBeClosed()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsTableColumnExistsSQL)).WithArgs("admin_user", "email").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAvailableAdminCandidatesWithEmailSQL)).WithArgs(int64(7), int64(11), 3).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(11, "user11", "name11", "nick11", "", "111").AddRow(9, "user9", "name9", "nick9", "", "999").AddRow(7, "user7", "name7", "nick7", "", "777")).RowsWillBeClosed()
	list, cursor, more, err := repo.ListAvailableAdminCandidates(context.Background(), 0, 2)
	if err != nil || !more || cursor != "9" || len(list) != 2 || list[1]["id"] != "9" {
		t.Fatalf("first page: list=%v cursor=%s more=%t err=%v", list, cursor, more, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAvailableAdminCandidateWindowAfterSQL)).WithArgs(int64(9), 100).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7)).RowsWillBeClosed()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAvailableAdminCandidatesWithEmailSQL)).WithArgs(int64(7), int64(7), 3).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(7, "user7", "name7", "nick7", "", "777")).RowsWillBeClosed()
	list, cursor, more, err = repo.ListAvailableAdminCandidates(context.Background(), 9, 2)
	if err != nil || more || cursor != "" || len(list) != 1 || list[0]["id"] != "7" {
		t.Fatalf("last page: list=%v cursor=%s more=%t err=%v", list, cursor, more, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestHGListAvailableAdminCandidatesEmptyWindow 即使窗口内全是管理员，也能返回继续翻页的游标，且不做无界扫描。
func TestHGListAvailableAdminCandidatesEmptyWindow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ids := sqlmock.NewRows([]string{"id"})
	for id := 100; id > 0; id-- {
		ids.AddRow(id)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAvailableAdminCandidateWindowSQL)).WithArgs(100).WillReturnRows(ids).RowsWillBeClosed()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsTableColumnExistsSQL)).WithArgs("admin_user", "email").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAvailableAdminCandidatesSQL)).WithArgs(int64(1), int64(100), 11).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_name", "nickname", "email", "phone"})).RowsWillBeClosed()
	list, cursor, more, err := NewRepository(db).ListAvailableAdminCandidates(context.Background(), 0, 10)
	if err != nil || len(list) != 0 || cursor != "1" || !more {
		t.Fatalf("empty window: list=%v cursor=%s more=%t err=%v", list, cursor, more, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

package OpsServicePackage

import (
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestHGSearchAdminCandidatesEmptyKeyword 验证空关键词分页续页协议与错误游标在访问数据库前被拒绝。
func TestHGSearchAdminCandidatesEmptyKeyword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(OpsRepositoryPackage.NewRepository(db), nil, nil)
	cols := []string{"id", "user_id", "user_name", "nickname", "email", "phone"}
	window := sqlmock.NewRows([]string{"id"})
	for id := 110; id >= 11; id-- {
		window.AddRow(id)
	}
	candidates := sqlmock.NewRows(cols)
	for id := 110; id >= 100; id-- {
		candidates.AddRow(id, "uid", "name", "nick", "", "123")
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAvailableAdminCandidateWindowSQL)).WithArgs(100).WillReturnRows(window).RowsWillBeClosed()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsTableColumnExistsSQL)).WithArgs("admin_user", "email").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAvailableAdminCandidatesSQL)).WithArgs(int64(11), int64(110), 11).WillReturnRows(candidates).RowsWillBeClosed()
	resp, err := service.SearchAdminCandidates(context.Background(), "  ", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.HasMore || resp.NextCursor != "101" || resp.Total != 10 || len(resp.List) != 10 {
		t.Fatalf("empty keyword: resp=%+v err=%v", resp, err)
	}
	if _, err := service.SearchAdminCandidates(context.Background(), "", "-1", 10); err == nil {
		t.Fatal("negative cursor accepted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

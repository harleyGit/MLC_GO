package OpsRepositoryPackage

import (
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

var hgOpsTestDriverOnce sync.Once

func TestAssignUserRolesMapsBusinessRoleIDsAndBatchInserts(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)

	err := repo.AssignUserRoles(context.Background(), "UID-101", []string{
		"ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8B",
		"ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8C",
		"ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8B",
	})
	if err != nil {
		t.Fatalf("AssignUserRoles returned error: %v", err)
	}
}

func TestGetUserRolesReadsUserRoleView(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)

	roles, err := repo.GetUserRoles(context.Background(), "UID-101")
	if err != nil {
		t.Fatalf("GetUserRoles returned error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("len(roles) = %d, want 1", len(roles))
	}
	if got := roles[0]["id"]; got != "ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8B" {
		t.Fatalf("role id = %v, want business role_id", got)
	}
	if got := roles[0]["name"]; got != "owner" {
		t.Fatalf("role name = %v, want owner", got)
	}
}

func TestGetRoleListReturnsBusinessRoleID(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)

	list, total, hasMore, err := repo.GetRoleList(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("GetRoleList returned error: %v", err)
	}
	if total != -1 {
		t.Fatalf("total = %d, want -1", total)
	}
	if hasMore {
		t.Fatalf("hasMore = true, want false")
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if got := list[0]["id"]; got != "ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8B" {
		t.Fatalf("role id = %v, want ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8B", got)
	}
	if got := list[0]["idInt"]; got != int64(101) {
		t.Fatalf("role cursor id = %v, want 101", got)
	}
}

func TestHasAssetPermissionUsesDatabaseRBAC(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)
	allowed, err := repo.HasAssetPermission(context.Background(), "UID-101", "asset.coin.grant")
	if err != nil {
		t.Fatalf("HasAssetPermission returned error: %v", err)
	}
	if !allowed {
		t.Fatal("HasAssetPermission = false, want true")
	}
}

func TestAppendAssetAuditWritesImmutableRecord(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)
	err := repo.AppendAssetAudit(context.Background(), OpsDtoPackage.HGAssetAuditRecord{OperatorID: "UID-101", Action: "coin.grant", TargetUserID: "UID-202", SourceIP: "203.0.113.8", RequestID: "REQ-1", TID: "TID-1", OldBalance: 4, NewBalance: 5, Outcome: "succeeded"})
	if err != nil {
		t.Fatalf("AppendAssetAudit returned error: %v", err)
	}
}

func TestCompleteCoinCorrectionRejectsLostStateTransition(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateOpsCoinCorrectionCompleteSQL)).
		WithArgs("applied", uint64(91), uint64(12), "", "applied", "COR-91", "admin-approver").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = NewRepository(db).CompleteCoinCorrection(context.Background(), "COR-91", "admin-approver", CoinModelPackage.HGMutationResult{TransactionID: 91, BalanceAfter: 12}, "")
	if err == nil {
		t.Fatal("expected zero-row state transition to fail")
	}
}

func TestAssignRolePermissionsReplacesBoundedPermissionSetAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsRoleInternalIDForUpdateSQL)).WithArgs("ROLE-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DeleteOpsRolePermissionsSQL)).WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsPermissionIDByCodeSQL)).WithArgs("asset.coin.grant").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOpsRolePermissionSQL)).WithArgs(int64(7), int64(11), "admin-1", "admin-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = NewRepository(db).AssignRolePermissions(context.Background(), "admin-1", "ROLE-1", nil, []string{"asset.coin.grant", "asset.coin.grant"})
	if err != nil {
		t.Fatalf("AssignRolePermissions() error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchAdminUsersFindsAdminByLinkedUsersUserName(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)

	list, total, err := repo.SearchAdminUsers(context.Background(), "alice", 10)
	if err != nil {
		t.Fatalf("SearchAdminUsers returned error: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if got := list[0]["id"]; got != "UID-101" {
		t.Fatalf("admin id = %v, want UID-101", got)
	}
	if got := list[0]["name"]; got != "Alice Admin" {
		t.Fatalf("admin name = %v, want Alice Admin", got)
	}
}

func TestSearchAdminUsersFindsAdminByLinkedUsersEmail(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)

	list, total, err := repo.SearchAdminUsers(context.Background(), "alice@example.com", 10)
	if err != nil {
		t.Fatalf("SearchAdminUsers returned error: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("got total=%d len=%d, want 1 result", total, len(list))
	}
	if got := list[0]["id"]; got != "UID-101" {
		t.Fatalf("admin id = %v, want UID-101", got)
	}
}

func TestSearchAdminUsersFindsAdminByLinkedUsersNickName(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)

	list, total, err := repo.SearchAdminUsers(context.Background(), "黄诗", 10)
	if err != nil {
		t.Fatalf("SearchAdminUsers returned error: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("got total=%d len=%d, want 1 result", total, len(list))
	}
	if got := list[0]["id"]; got != "UID-101" {
		t.Fatalf("admin id = %v, want UID-101", got)
	}
}

func TestSearchAdminUsersFindsAdminByLinkedUsersPhone(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)

	list, total, err := repo.SearchAdminUsers(context.Background(), "13800138000", 10)
	if err != nil {
		t.Fatalf("SearchAdminUsers returned error: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("got total=%d len=%d, want 1 result", total, len(list))
	}
	if got := list[0]["id"]; got != "UID-101" {
		t.Fatalf("admin id = %v, want UID-101", got)
	}
}

func TestSearchAdminUsersFindsAdminByPartialLinkedUsersPhone(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)

	list, total, err := repo.SearchAdminUsers(context.Background(), "176", 10)
	if err != nil {
		t.Fatalf("SearchAdminUsers returned error: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("got total=%d len=%d, want 1 result", total, len(list))
	}
	if got := list[0]["id"]; got != "UID-101" {
		t.Fatalf("admin id = %v, want UID-101", got)
	}
}

func TestSearchAdminUsersFindsAdminByPartialAdminMobile(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)

	list, total, err := repo.SearchAdminUsers(context.Background(), "3800", 10)
	if err != nil {
		t.Fatalf("SearchAdminUsers returned error: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("got total=%d len=%d, want 1 result", total, len(list))
	}
	if got := list[0]["id"]; got != "UID-101" {
		t.Fatalf("admin id = %v, want UID-101", got)
	}
}

func TestSearchAdminUsersFindsAdminByAdminID(t *testing.T) {
	db := newHGTestDB(t)
	repo := NewRepository(db)

	list, total, err := repo.SearchAdminUsers(context.Background(), "101", 10)
	if err != nil {
		t.Fatalf("SearchAdminUsers returned error: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("got total=%d len=%d, want 1 result", total, len(list))
	}
	if got := list[0]["id"]; got != "UID-101" {
		t.Fatalf("admin id = %v, want UID-101", got)
	}
}

func newHGTestDB(t *testing.T) *sql.DB {
	t.Helper()
	hgOpsTestDriverOnce.Do(func() {
		sql.Register("hg_ops_repository_test", hgOpsTestDriver{})
	})
	db, err := sql.Open("hg_ops_repository_test", "")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type hgOpsTestDriver struct{}

func (hgOpsTestDriver) Open(string) (driver.Conn, error) {
	return hgOpsTestConn{}, nil
}

type hgOpsTestConn struct{}

func (hgOpsTestConn) Prepare(query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected prepared statement: %s", query)
}
func (hgOpsTestConn) Close() error              { return nil }
func (hgOpsTestConn) Begin() (driver.Tx, error) { return hgOpsTestTx{}, nil }

func (hgOpsTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return hgOpsTestTx{}, nil
}

func (hgOpsTestConn) PrepareContext(_ context.Context, query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected prepared statement: %s", query)
}

type hgOpsTestTx struct{}

func (hgOpsTestTx) Commit() error   { return nil }
func (hgOpsTestTx) Rollback() error { return nil }

type hgOpsTestResult int64

func (r hgOpsTestResult) LastInsertId() (int64, error) { return 0, nil }
func (r hgOpsTestResult) RowsAffected() (int64, error) { return int64(r), nil }

func (hgOpsTestConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	switch {
	case strings.Contains(query, "DELETE FROM `admin_user_role`") && len(args) == 1 && args[0] == int64(101):
		return hgOpsTestResult(1), nil
	case strings.Contains(query, "DELETE FROM `user_role_view`") && len(args) == 1 && args[0] == int64(101):
		return hgOpsTestResult(1), nil
	case strings.Contains(query, "INSERT INTO `admin_user_role`") && strings.Count(query, "(?, ?, NOW(), 0)") == 2:
		want := []driver.Value{int64(101), int64(101), int64(101), int64(102)}
		if len(args) != len(want) {
			return nil, fmt.Errorf("insert args len = %d, want %d", len(args), len(want))
		}
		for i := range want {
			if args[i] != want[i] {
				return nil, fmt.Errorf("insert arg[%d] = %v, want %v", i, args[i], want[i])
			}
		}
		return hgOpsTestResult(2), nil
	case strings.Contains(query, "INSERT INTO `user_role_view`"):
		want := []driver.Value{int64(101), "ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8B", "owner", int64(1), int64(101), "ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8C", "auditor", int64(1)}
		if got := strings.Count(query, "?"); got != len(want) {
			return nil, fmt.Errorf("view insert placeholder count = %d, want %d", got, len(want))
		}
		if len(args) != len(want) {
			return nil, fmt.Errorf("view insert args len = %d, want %d", len(args), len(want))
		}
		for i := range want {
			if args[i] != want[i] {
				return nil, fmt.Errorf("view insert arg[%d] = %v, want %v", i, args[i], want[i])
			}
		}
		return hgOpsTestResult(2), nil
	case strings.Contains(query, "INSERT INTO `ops_asset_audit`"):
		if len(args) != 12 || args[0] != "UID-101" || args[1] != "coin.grant" || args[2] != "UID-202" || args[3] != "203.0.113.8" || args[4] != "REQ-1" || args[5] != "TID-1" || args[6] != int64(4) || args[7] != int64(5) || args[10] != "succeeded" {
			return nil, fmt.Errorf("unexpected audit args: %v", args)
		}
		return hgOpsTestResult(1), nil
	default:
		return nil, fmt.Errorf("unexpected exec: %s args=%v", query, args)
	}
}

func (c hgOpsTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	values := make([]driver.Value, 0, len(args))
	for _, arg := range args {
		values = append(values, arg.Value)
	}
	return c.Exec(query, values)
}

func (hgOpsTestConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "INFORMATION_SCHEMA.COLUMNS"):
		return newHGTestRows([]string{"COUNT(*)"}, [][]driver.Value{{int64(1)}}), nil
	case strings.Contains(query, "FROM `user_role_view`"):
		return newHGTestRows([]string{"role_id", "role_name", "status", "create_at"}, [][]driver.Value{{"ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8B", "owner", int64(1), time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)}}), nil
	case strings.Contains(query, "FROM `role`") && strings.Contains(query, "`role_id` IN"):
		return newHGTestRows([]string{"role_id", "id", "name", "status"}, [][]driver.Value{{"ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8B", int64(101), "owner", int64(1)}, {"ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8C", int64(102), "auditor", int64(1)}}), nil
	case strings.Contains(query, "FROM `role`") && strings.Contains(query, "SELECT `role_id`"):
		return newHGTestRows([]string{"role_id", "id", "name", "description", "create_at"}, [][]driver.Value{{"ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8B", int64(101), "owner", "Owner role", time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)}}), nil
	case strings.Contains(query, "FROM `role`"):
		return newHGTestRows([]string{"id", "name", "description", "create_at"}, [][]driver.Value{{int64(101), "owner", "Owner role", time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)}}), nil
	case strings.Contains(query, "FROM `admin_user`") && strings.Contains(query, "`user_id` LIKE") && args[0] == "%101%":
		return newHGTestRows([]string{"user_id", "name", "nick_name", "email", "mobile", "status"}, [][]driver.Value{{"UID-101", "Alice Admin", "Alice", "alice@example.com", "13800138000", int64(1)}}), nil
	case strings.Contains(query, "SELECT `id` FROM `admin_user`") && strings.Contains(query, "`user_id` = ?") && args[0] == "UID-101":
		return newHGTestRows([]string{"id"}, [][]driver.Value{{int64(101)}}), nil
	case strings.Contains(query, "JOIN `role_permission`") && strings.Contains(query, "p.`code` = ?") && args[0] == "UID-101" && args[1] == "asset.coin.grant":
		return newHGTestRows([]string{"allowed"}, [][]driver.Value{{int64(1)}}), nil
	case strings.Contains(query, "FROM `admin_user`") && strings.Contains(query, "`mobile` LIKE") && args[0] == "%3800%":
		return newHGTestRows([]string{"user_id", "name", "nick_name", "email", "mobile", "status"}, [][]driver.Value{{"UID-101", "Alice Admin", "Alice", "alice@example.com", "13800138000", int64(1)}}), nil
	case strings.Contains(query, "FROM `admin_user`") && strings.Contains(query, "`user_id` = ?") && args[0] == "UID-101":
		return newHGTestRows([]string{"user_id", "name", "nick_name", "email", "mobile", "status"}, [][]driver.Value{{"UID-101", "Alice Admin", "Alice", "alice@example.com", "13800138000", int64(1)}}), nil
	case strings.Contains(query, "FROM `users`") && strings.Contains(query, "`user_name` LIKE") && args[0] == "%alice%":
		return newHGTestRows([]string{"user_id"}, [][]driver.Value{{"UID-101"}}), nil
	case strings.Contains(query, "FROM `users`") && strings.Contains(query, "`nickname` LIKE") && args[0] == "%黄诗%":
		return newHGTestRows([]string{"user_id"}, [][]driver.Value{{"UID-101"}}), nil
	case strings.Contains(query, "FROM `users`") && strings.Contains(query, "`email` LIKE") && args[0] == "%alice@example.com%":
		return newHGTestRows([]string{"user_id"}, [][]driver.Value{{"UID-101"}}), nil
	case strings.Contains(query, "FROM `users`") && strings.Contains(query, "`phone` LIKE") && args[0] == "%13800138000%":
		return newHGTestRows([]string{"user_id"}, [][]driver.Value{{"UID-101"}}), nil
	case strings.Contains(query, "FROM `users`") && strings.Contains(query, "`phone` LIKE") && args[0] == "%176%":
		return newHGTestRows([]string{"user_id"}, [][]driver.Value{{"UID-101"}}), nil
	case strings.Contains(query, "FROM `admin_user`"):
		return newHGTestRows([]string{"user_id", "name", "nick_name", "email", "mobile", "status"}, nil), nil
	default:
		return newHGTestRows([]string{"id"}, nil), nil
	}
}

func (c hgOpsTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	values := make([]driver.Value, 0, len(args))
	for _, arg := range args {
		values = append(values, arg.Value)
	}
	return c.Query(query, values)
}

type hgTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func newHGTestRows(columns []string, values [][]driver.Value) *hgTestRows {
	return &hgTestRows{columns: columns, values: values, index: -1}
}

func (r *hgTestRows) Columns() []string { return r.columns }
func (r *hgTestRows) Close() error      { return nil }

func (r *hgTestRows) Next(dest []driver.Value) error {
	r.index++
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	return nil
}

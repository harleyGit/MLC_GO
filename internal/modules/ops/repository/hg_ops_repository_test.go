package OpsRepositoryPackage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"
)

var hgOpsTestDriverOnce sync.Once

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

func (hgOpsTestConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (hgOpsTestConn) Close() error                        { return nil }
func (hgOpsTestConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (hgOpsTestConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "INFORMATION_SCHEMA.COLUMNS"):
		return newHGTestRows([]string{"COUNT(*)"}, [][]driver.Value{{int64(1)}}), nil
	case strings.Contains(query, "FROM `admin_user`") && strings.Contains(query, "`user_id` LIKE") && args[0] == "%101%":
		return newHGTestRows([]string{"user_id", "name", "nick_name", "email", "mobile", "status"}, [][]driver.Value{{"UID-101", "Alice Admin", "Alice", "alice@example.com", "13800138000", int64(1)}}), nil
	case strings.Contains(query, "FROM `admin_user`") && strings.Contains(query, "`user_id` = ?") && args[0] == "UID-101":
		return newHGTestRows([]string{"user_id", "name", "nick_name", "email", "mobile", "status"}, [][]driver.Value{{"UID-101", "Alice Admin", "Alice", "alice@example.com", "13800138000", int64(1)}}), nil
	case strings.Contains(query, "FROM `users`") && strings.Contains(query, "`user_name` LIKE") && args[0] == "%alice%":
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

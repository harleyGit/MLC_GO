package repository

import (
	SQLQueriesPackage "MLC_GO/internal/infrastructure/persistence/mysql/queries"
	"strings"
	"testing"
)

func TestNewUserRepo_NilDBPanics(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic when db is nil")
		}
	}()

	_ = NewUserRepo(nil)
}

func TestUserSQL_IDSemantics(t *testing.T) {
	if !strings.Contains(SQLQueriesPackage.GetUserByIDSQL, "WHERE id = ?") {
		t.Fatalf("GetUserByIDSQL must query by primary id, sql=%s", SQLQueriesPackage.GetUserByIDSQL)
	}
	if !strings.Contains(SQLQueriesPackage.GetUserByUserIDSQL, "WHERE user_id = ?") {
		t.Fatalf("GetUserByUserIDSQL must query by business user_id, sql=%s", SQLQueriesPackage.GetUserByUserIDSQL)
	}
	if !strings.Contains(SQLQueriesPackage.UpdateUserInfoByIDSQL, "WHERE id = ?") {
		t.Fatalf("UpdateUserInfoByIDSQL must update by primary id, sql=%s", SQLQueriesPackage.UpdateUserInfoByIDSQL)
	}
	if !strings.Contains(SQLQueriesPackage.UpdateUserInfoByUserIDSQL, "WHERE user_id = ?") {
		t.Fatalf("UpdateUserInfoByUserIDSQL must update by business user_id, sql=%s", SQLQueriesPackage.UpdateUserInfoByUserIDSQL)
	}
	if strings.Contains(SQLQueriesPackage.UpdateUserInfoByIDSQL, ", WHERE") {
		t.Fatalf("UpdateUserInfoByIDSQL contains invalid comma before WHERE, sql=%s", SQLQueriesPackage.UpdateUserInfoByIDSQL)
	}
	if strings.Contains(SQLQueriesPackage.UpdateUserInfoByUserIDSQL, ", WHERE") {
		t.Fatalf("UpdateUserInfoByUserIDSQL contains invalid comma before WHERE, sql=%s", SQLQueriesPackage.UpdateUserInfoByUserIDSQL)
	}
	if !strings.Contains(SQLQueriesPackage.SelectUserInfoByPhoneSQL, "user_id") {
		t.Fatalf("SelectUserInfoByPhoneSQL must select user_id for token issue, sql=%s", SQLQueriesPackage.SelectUserInfoByPhoneSQL)
	}
}

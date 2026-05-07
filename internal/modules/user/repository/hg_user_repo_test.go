/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-21 15:13:27
  - @LastEditors: GangHuang harleysor@qq.com
  - @LastEditTime: 2026-01-21 18:14:30

* @FilePath: /MLC_GO/internal/modules/user/repository/hg_user_repo_test.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

*
*/
package UserRepositoryPackage

import (
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

/* 测试DB连接 */
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open(
		"mysql",
		"test:test@tcp(127.0.0.1:3306)/test_db?parseTime=true",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("skip mysql integration test: %v", err)
	}
	return db
}

func skipIfTestDBUnavailable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	errText := err.Error()
	if strings.Contains(errText, "Access denied") || strings.Contains(errText, "Unknown database") || strings.Contains(errText, "connection refused") {
		t.Skipf("skip mysql integration test: %v", err)
	}
	t.Fatal(err)
}

/* Insert + Get（NULL 字段测试） */
func TestUserRepo_InsertAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserRepo(db)
	ctx := context.Background()

	user := &UserModelsPackage.HGUserModel{
		Email:        sql.NullString{Valid: false},
		Phone:        sql.NullString{String: "13800000000", Valid: true},
		PasswordHash: sql.NullString{String: "hash", Valid: true},
		Salt:         sql.NullString{String: "salt", Valid: true},
	}

	err := repo.Insert(ctx, user)
	if err != nil {
		skipIfTestDBUnavailable(t, err)
	}

	got, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Email.Valid {
		t.Fatal("email should be NULL")
	}

	if !got.Phone.Valid || got.Phone.String != "13800000000" {
		t.Fatal("phone mismatch")
	}
}

/* Patch 场景 */
func TestUserRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewUserRepo(db)
	ctx := context.Background()

	user := &UserModelsPackage.HGUserModel{
		Email:        sql.NullString{String: "a@b.com", Valid: true},
		PasswordHash: sql.NullString{String: "hash", Valid: true},
		Salt:         sql.NullString{String: "salt", Valid: true},
	}

	if err := repo.Insert(ctx, user); err != nil {
		skipIfTestDBUnavailable(t, err)
	}

	user.Email = sql.NullString{String: "", Valid: true}

	err := repo.Update(ctx, user)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := repo.GetByID(ctx, user.ID)
	if !got.Email.Valid || got.Email.String != "" {
		t.Fatal("email update failed")
	}
}

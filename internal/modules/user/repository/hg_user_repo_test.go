/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-21 15:13:27
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 15:38:45

* @FilePath: /MLC_GO/internal/modules/user/repository/hg_user_repo_test.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

*
*/
package UserRepositoryPackage

import (
	UserModelsPackage "MLC_GO/internal/models/user_models"
	"context"
	"database/sql"
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
	return db
}

/* Insert + Get（NULL 字段测试） */
func TestUserRepo_InsertAndGet(t *testing.T) {
    db := setupTestDB(t)
    repo := NewUserRepo(db)
    ctx := context.Background()

    user := &UserModelsPackage.HGUserModel{
        Phone: "13800000000",
        PasswordHash: "hash",
        Salt:         "salt",
    }

    err := repo.Insert(ctx, user)
    if err != nil {
        t.Fatal(err)
    }

    got, err := repo.GetByID(ctx, user.UserID)
    if err != nil {
        t.Fatal(err)
    }

    if got.Email == "" {
        t.Fatal("email should be NULL")
    }

    if  got.Phone != "13800000000" {
        t.Fatal("phone mismatch")
    }
}

/* Patch 场景 */
func TestUserRepo_Update(t *testing.T) {
    db := setupTestDB(t)
    repo := NewUserRepo(db)
    ctx := context.Background()

    user := &UserModelsPackage.HGUserModel{
        Email: "a@b.com",
        PasswordHash: "hash",
        Salt:         "salt",
    }

    _ = repo.Insert(ctx, user)

    user.Email = ""

    err := repo.Update(ctx, user)
    if err != nil {
        t.Fatal(err)
    }

    got, _ := repo.GetByID(ctx, user.UserID)
    if got.Email != "" {
        t.Fatal("email update failed")
    }
}



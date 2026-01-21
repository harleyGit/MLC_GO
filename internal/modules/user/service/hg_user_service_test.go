/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 15:28:45
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 15:39:01
 * @FilePath: /MLC_GO/internal/modules/user/service/hg_user_service_test.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserServicePackage

import (
	UserModelsPackage "MLC_GO/internal/models/user_models"
	UserRepositoryPackage "MLC_GO/internal/modules/user/repository"
	"context"
	"database/sql"
	"testing"
)

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

func TestUserRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := UserRepositoryPackage.NewUserRepo(db)
	ctx := context.Background()

	user := &UserModelsPackage.HGUserModel{
		Email:        "a@b.com",
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

/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 15:28:45
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-05 09:38:45
 * @FilePath: /MLC_GO/internal/modules/user/service/hg_user_service_test.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserServicePackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserRepositoryPackage "MLC_GO/internal/modules/user/repository"
	"context"
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

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
func TestCreateUser_NullEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := UserRepositoryPackage.NewUserRepo(db)
	svc := NewUserService(repo, nil, nil)

	phone := "13800000000"
	d := &UserDtoPackage.HGCreateUserDTO{
		Email:    nil, // 未传
		Phone:    &phone,
		Password: "123456",
	}

	err := svc.CreateUser(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
}

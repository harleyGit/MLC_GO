/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 10:03:08
 * @LastEditors: Harley harelysoa@qq.com
 * @LastEditTime: 2026-01-20 23:22:17
 * @FilePath: /MLC_GO/internal/modules/user/repository/hg_user_respository.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserRepositoryPackage

import (
	UserModelsPackage "MLC_GO/internal/models/user_models"
	"context"
	"database/sql"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Insert(ctx context.Context, u *UserModelsPackage.HGUserModel) error {
	res, err := r.db.ExecContext(
		ctx,
		``,

	)
	u.UserID, _ = res.LastInsertId()
	return err
}
/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 10:03:08
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 10:52:34
 * @FilePath: /MLC_GO/internal/modules/user/repository/hg_user_respository.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/infrastructure/persistence/mysql/queries"
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

// TODO：若是插入后还需要操作其他表，因包裹在事务中【这个事务指的是啥】
// 插入用户信息
func (r *UserRepo) Insert(ctx context.Context, u *UserModelsPackage.HGUserModel) error {
	// ExecContext 用于执行一条“写操作”SQL，用于 插入、更新、删除操作
	// TODO： 检查Phone和Email唯一性，上层操作判断
	res, err := r.db.ExecContext(
		ctx,
		SQLQueriesPackage.InsertUserInfoSQL,
		u.Email,
		u.Phone,
		u.PasswordHash,
		u.Salt,
	)
	// TODO：可能失败，失败比如不支持数据库特性【这个特性值的是什么】？需要解决下
	if err != nil {
		return  err
	}
	u.UserID, _ = res.LastInsertId()
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*UserModelsPackage.HGUserModel, error) {
	var u UserModelsPackage.HGUserModel
	err := r.db.QueryRowContext(
		ctx,
		SQLQueriesPackage.GetUserByIDSQL,
		id,
	).Scan(
		&u.UserID,
		&u.Username,
		&u.Email,
		&u.Phone,
		&u.PasswordHash,
		&u.Salt,
	)

	return  &u, err
}

func (r *UserRepo) Update(ctx context.Context, u *UserModelsPackage.HGUserModel) error {
	_, error := r.db.ExecContext(
		ctx,
		SQLQueriesPackage.UpdateUserInfoSQL,
		u.Email,
		u.Phone,
		u.UserID,
	)
	return  error
}
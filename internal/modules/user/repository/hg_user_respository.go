/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 10:03:08
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-25 21:05:17
 * @FilePath: /MLC_GO/internal/modules/user/repository/hg_user_respository.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/infrastructure/persistence/mysql/queries"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	RepositoryPackage "MLC_GO/internal/repository"
	"context"
	"database/sql"
)

/* UserRepo 继承  RepositoryPackage.HGBaseRepo */
type UserRepo struct {
	*RepositoryPackage.HGBaseRepo
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{HGBaseRepo: RepositoryPackage.NewBaseRepo(db)}
}

// TODO：若是插入后还需要操作其他表，因包裹在事务中【这个事务指的是啥】
// 插入用户信息
func (r *UserRepo) Insert(ctx context.Context, u *UserModelsPackage.HGUserModel) error {
	// ExecContext 用于执行一条“写操作”SQL，用于 插入、更新、删除操作
	// TODO： 检查Phone和Email唯一性，上层操作判断
	res, err := r.Exec(
		ctx,
		SQLQueriesPackage.InsertUserInfoSQL,
		u.Email,
		u.Phone,
		u.PasswordHash,
		u.Salt,
	)
	// TODO：可能失败，失败比如不支持数据库特性【这个特性值的是什么】？需要解决下
	if err != nil {
		// TODO: 若是失败需要回滚，比如：tx， err ：= r。db。GeginTx（ctx， nil）；
		// TODO：res， err ：= tx.ExecContext（ctx， sql语句） tx.Rollback()
		// TODO:可以保持事务一致性
		return err
	}
	u.ID, _ = res.LastInsertId()
	return err
}

func (r *UserRepo) GetByPhone(ctx context.Context, phone string) (*UserModelsPackage.HGUserModel, error) {
	var u UserModelsPackage.HGUserModel
	err := r.QueryRow(
		ctx,
		SQLQueriesPackage.SelectUserInfoByPhoneSQL,
		phone,
	).Scan(
		&u.ID,
		&u.Email,
		&u.Phone,
		&u.PasswordHash,
		&u.Salt,
	)

	return &u, err
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*UserModelsPackage.HGUserModel, error) {
	var u UserModelsPackage.HGUserModel
	err := r.QueryRow(
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

	return &u, err
}

func (r *UserRepo) Update(ctx context.Context, u *UserModelsPackage.HGUserModel) error {
	_, error := r.Exec(
		ctx,
		SQLQueriesPackage.UpdateUserInfoSQL,
		u.Email,
		u.Phone,
		u.UserID,
	)
	return error
}


func (r *UserRepo) FindPage(
	ctx context.Context,
	page, size int,
) ([]UserModelsPackage.HGUserModel, int, error) {

	offset := (page - 1) * size

	// 查询总数
	var total int
	err := r.QueryRow(
		ctx,
		SQLQueriesPackage.UserTotalNumSQL,
	).Scan(
		&total,
	)
	if err != nil {
		return nil, 0, err
	}

	// 查询分页数据
	rows, err := r.QueryContext(
		ctx,
		SQLQueriesPackage.QueryUserPageSQL,
		size,
		offset,
	)
	if err != nil {
		return  nil, 0, err
	}
	defer rows.Close()

	var users []UserModelsPackage.HGUserModel
	for rows.Next() {
		var u UserModelsPackage.HGUserModel
		err := rows.Scan(
			&u.UserID,
			&u.Username,
			&u.Email,
			&u.Phone,
			&u.PasswordHash,
			&u.Salt,
		)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}

	return users, total, nil

}

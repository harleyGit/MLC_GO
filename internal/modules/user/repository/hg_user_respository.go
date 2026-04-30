/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 10:03:08
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-03-01 22:29:27
 * @FilePath: /MLC_GO/internal/modules/user/repository/hg_user_respository.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/infrastructure/persistence/mysql/queries"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	RepositoryPackage "MLC_GO/internal/repository"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
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
	
	// 使用带超时的上下文，防止长时间阻塞
	const queryTimeout = 5 * time.Second
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	
	res, err := r.Exec(
		queryCtx,
		SQLQueriesPackage.InsertUserInfoSQL,
		u.Email,
		u.Phone,
		u.PasswordHash,
		u.Salt,
	)
	// TODO：可能失败，失败比如不支持数据库特性【这个特性值的是什么】？需要解决下
	if err != nil {
		// 检查是否是上下文取消错误
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("insert user operation was canceled: %w", err)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("insert user operation timed out after %v: %w", queryTimeout, err)
		}
		// TODO: 若是失败需要回滚，比如：tx， err ：= r。db。GeginTx（ctx， nil）；
		// TODO：res， err ：= tx.ExecContext（ctx， sql语句） tx.Rollback()
		// TODO:可以保持事务一致性
		return fmt.Errorf("failed to insert user: %w", err)
	}
	u.ID, _ = res.LastInsertId()
	return nil
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
	// 使用带超时的上下文，防止长时间阻塞
	const queryTimeout = 5 * time.Second
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	
	_, err := r.Exec(
		queryCtx,
		SQLQueriesPackage.UpdateUserInfoSQL,
		u.Email,
		u.Phone,
		u.UserID,
	)
	if err != nil {
		// 检查是否是上下文取消错误
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("update user operation was canceled: %w", err)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("update user operation timed out after %v: %w", queryTimeout, err)
		}
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
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
		return nil, 0, err
	}
	defer rows.Close()

	var users []UserModelsPackage.HGUserModel
	for rows.Next() {
		var u UserModelsPackage.HGUserModel
		err := rows.Scan(
			&u.ID,
			&u.UserID,
			&u.Username,
			&u.Email,
			&u.Phone,
			&u.PasswordHash,
			&u.Salt,
			&u.CreatedAt,
			&u.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}

	return users, total, nil

}

// FindByCursor 使用主键 id 倒序做 cursor 分页，避免大 offset 深分页扫描。
// 查询时多取一条，用来判断是否还有下一页，并计算 nextCursor。
func (r *UserRepo) FindByCursor(
	ctx context.Context,
	cursor int64,
	size int,
) ([]UserModelsPackage.HGUserModel, int64, bool, error) {
	limit := size + 1

	var (
		rows *sql.Rows
		err  error
	)

	if cursor > 0 {
		rows, err = r.QueryContext(
			ctx,
			SQLQueriesPackage.QueryUserPageV2SQL,
			cursor,
			limit,
		)
	} else {
		rows, err = r.QueryContext(
			ctx,
			SQLQueriesPackage.QueryUserPageFirstSQL,
			limit,
		)
	}
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()

	users := make([]UserModelsPackage.HGUserModel, 0, limit)
	for rows.Next() {
		var u UserModelsPackage.HGUserModel
		err = rows.Scan(
			&u.ID,
			&u.UserID,
			&u.Username,
			&u.Email,
			&u.Phone,
			&u.PasswordHash,
			&u.Salt,
			&u.CreatedAt,
			&u.UpdatedAt,
		)
		if err != nil {
			return nil, 0, false, err
		}
		users = append(users, u)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, false, err
	}

	hasMore := len(users) > size
	if hasMore {
		users = users[:size]
	}

	var nextCursor int64
	if hasMore && len(users) > 0 {
		nextCursor = users[len(users)-1].ID
	}

	return users, nextCursor, hasMore, nil
}

func (r *UserRepo) CountUsers(ctx context.Context) (int, error) {
	var total int
	err := r.QueryRow(
		ctx,
		SQLQueriesPackage.UserTotalNumSQL,
	).Scan(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
}

// UpdateProfileByID 按传入字段动态更新用户资料，支持单字段或多字段修改。
func (r *UserRepo) UpdateProfileByID(
	ctx context.Context,
	userID string,
	d *UserDtoPackage.HGUpdateUserProfileReqDTO,
) error {
	if d == nil || !d.HasAnyField() {
		return errors.New("no fields to update")
	}

	setClauses := make([]string, 0, 5)
	args := make([]any, 0, 6)

	if d.Nickname != nil {
		setClauses = append(setClauses, "`nickname` = ?")
		args = append(args, *d.Nickname)
	}
	if d.Signature != nil {
		setClauses = append(setClauses, "`signature` = ?")
		args = append(args, *d.Signature)
	}
	if d.Gender != nil {
		setClauses = append(setClauses, "`gender` = ?")
		args = append(args, *d.Gender)
	}
	if d.BirthDate != nil {
		setClauses = append(setClauses, "`birth_month` = ?")
		args = append(args, *d.BirthDate)
	}
	if d.AvatarURL != nil {
		setClauses = append(setClauses, "`avatar_url` = ?")
		args = append(args, *d.AvatarURL)
	}

	query := fmt.Sprintf("UPDATE users SET %s WHERE user_id = ?", strings.Join(setClauses, ", "))
	args = append(args, userID)

	// 使用带超时的上下文，防止长时间阻塞
	const queryTimeout = 5 * time.Second
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	res, err := r.Exec(queryCtx, query, args...)
	if err != nil {
		// 检查是否是上下文取消错误
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("update profile operation was canceled: %w", err)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("update profile operation timed out after %v: %w", queryTimeout, err)
		}
		return fmt.Errorf("failed to update profile: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

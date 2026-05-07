/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 10:03:08
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-03-01 22:29:27
 * @FilePath: /MLC_GO/internal/modules/user/repository/hg_user_repository.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package repository

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

const userRepoQueryTimeout = 5 * time.Second

/* UserRepo 继承  RepositoryPackage.HGBaseRepo */
type UserRepo struct {
	*RepositoryPackage.HGBaseRepo
}

// NewUserRepo 创建用户仓储，集中封装 users 表 SQL 访问。
func NewUserRepo(db *sql.DB) *UserRepo {
	if db == nil {
		panic("user repository requires sql db")
	}
	return &UserRepo{HGBaseRepo: RepositoryPackage.NewBaseRepo(db)}
}

// Insert 插入用户基础认证信息，并回填自增主键 ID。
func (r *UserRepo) Insert(ctx context.Context, u *UserModelsPackage.HGUserModel) error {
	queryCtx, cancel := context.WithTimeout(ctx, userRepoQueryTimeout)
	defer cancel()

	res, err := r.Exec(
		queryCtx,
		SQLQueriesPackage.InsertUserSQL,
		u.UserID,
		u.Username,
		u.Email,
		u.Phone,
		u.PasswordHash,
		u.Salt,
	)
	if err != nil {
		return wrapUserRepoWriteErr("insert user", err)
	}
	if u.ID, err = res.LastInsertId(); err != nil {
		return fmt.Errorf("get inserted user id: %w", err)
	}
	return nil
}

// GetByPhone 根据手机号查询认证所需用户信息。
func (r *UserRepo) GetByPhone(ctx context.Context, phone string) (*UserModelsPackage.HGUserModel, error) {
	var u UserModelsPackage.HGUserModel
	err := r.QueryRow(
		ctx,
		SQLQueriesPackage.SelectUserInfoByPhoneSQL,
		phone,
	).Scan(
		&u.ID,
		&u.UserID,
		&u.Email,
		&u.Phone,
		&u.PasswordHash,
		&u.Salt,
	)

	return &u, err
}

// GetByID 根据自增主键 ID 查询用户资料。
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*UserModelsPackage.HGUserModel, error) {
	var u UserModelsPackage.HGUserModel
	err := r.QueryRow(
		ctx,
		SQLQueriesPackage.GetUserByIDSQL,
		id,
	).Scan(
		&u.ID,
		&u.UserID,
		&u.Username,
		&u.Nickname,
		&u.Signature,
		&u.Gender,
		&u.BirthMonth,
		&u.AvatarURL,
		&u.Email,
		&u.Phone,
	)

	return &u, err
}

// GetByUserID 根据 user_id 字符串查询用户（直接使用 VARCHAR 类型的 user_id 字段）
func (r *UserRepo) GetByUserID(ctx context.Context, userID string) (*UserModelsPackage.HGUserModel, error) {
	var u UserModelsPackage.HGUserModel
	err := r.QueryRow(
		ctx,
		SQLQueriesPackage.GetUserByUserIDSQL,
		userID,
	).Scan(
		&u.ID,
		&u.UserID,
		&u.Username,
		&u.Nickname,
		&u.Signature,
		&u.Gender,
		&u.BirthMonth,
		&u.AvatarURL,
		&u.Email,
		&u.Phone,
	)

	return &u, err
}

// GetByEmailOrPhone 根据邮箱或手机号查询用户
func (r *UserRepo) GetByEmailOrPhone(ctx context.Context, account string) (*UserModelsPackage.HGUserModel, error) {
	var u UserModelsPackage.HGUserModel
	err := r.QueryRow(
		ctx,
		SQLQueriesPackage.GetUserByEmailOrPhoneSQL,
		account,
		account,
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

// Update 按自增主键更新用户邮箱和手机号，仅用于内部主键明确的历史调用。
func (r *UserRepo) Update(ctx context.Context, u *UserModelsPackage.HGUserModel) error {
	queryCtx, cancel := context.WithTimeout(ctx, userRepoQueryTimeout)
	defer cancel()

	res, err := r.Exec(
		queryCtx,
		SQLQueriesPackage.UpdateUserInfoByIDSQL,
		u.Email,
		u.Phone,
		u.ID,
	)
	if err != nil {
		return wrapUserRepoWriteErr("update user", err)
	}
	return ensureRowsAffected(res)
}

// UpdateByUserID 按业务 user_id 更新用户邮箱和手机号，供对外用户资料接口使用。
func (r *UserRepo) UpdateByUserID(ctx context.Context, userID string, u *UserModelsPackage.HGUserModel) error {
	queryCtx, cancel := context.WithTimeout(ctx, userRepoQueryTimeout)
	defer cancel()

	res, err := r.Exec(
		queryCtx,
		SQLQueriesPackage.UpdateUserInfoByUserIDSQL,
		u.Email,
		u.Phone,
		userID,
	)
	if err != nil {
		return wrapUserRepoWriteErr("update user by user_id", err)
	}
	return ensureRowsAffected(res)
}

// FindPage 使用 offset 分页查询用户列表，保留用于历史调用。
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

	users := make([]UserModelsPackage.HGUserModel, 0, size)
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
	if err = rows.Err(); err != nil {
		return nil, 0, err
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

// CountUsers 查询用户总数，供列表分页响应和 total 缓存回源使用。
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

// UpdateProfileByUserID 按业务 user_id 动态更新用户资料，支持单字段或多字段修改。
func (r *UserRepo) UpdateProfileByUserID(
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

	queryCtx, cancel := context.WithTimeout(ctx, userRepoQueryTimeout)
	defer cancel()

	res, err := r.Exec(queryCtx, query, args...)
	if err != nil {
		return wrapUserRepoWriteErr("update profile", err)
	}

	return ensureRowsAffected(res)
}

// UpdateProfileByID 兼容旧方法名；实际语义为按业务 user_id 更新资料。
// Deprecated: 新代码使用 UpdateProfileByUserID，避免和自增主键 id 混淆。
func (r *UserRepo) UpdateProfileByID(
	ctx context.Context,
	userID string,
	d *UserDtoPackage.HGUpdateUserProfileReqDTO,
) error {
	return r.UpdateProfileByUserID(ctx, userID, d)
}

// ensureRowsAffected 将没有命中用户的更新统一转成 sql.ErrNoRows，便于 handler 映射 404。
func ensureRowsAffected(res sql.Result) error {
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// wrapUserRepoWriteErr 统一包装写操作错误，保留 context 取消和超时语义。
func wrapUserRepoWriteErr(operation string, err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s operation was canceled: %w", operation, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s operation timed out after %v: %w", operation, userRepoQueryTimeout, err)
	}
	return fmt.Errorf("failed to %s: %w", operation, err)
}

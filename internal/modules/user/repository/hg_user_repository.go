/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 10:03:08
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-03 20:36:52
 * @FilePath: /MLC_GO/internal/modules/user/repository/hg_user_repository.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package repository

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	RepositoryPackage "MLC_GO/internal/repository"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

const userRepoQueryTimeout = 5 * time.Second

// ErrUserSecurityDuplicate 表示邮箱、手机、QQ 或微信号已被其他安全记录占用。
var ErrUserSecurityDuplicate = errors.New("user security value duplicated")

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
		args = append(args, d.BirthDate.Value)
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

// UpdateSecurityByUserID 按业务 user_id 更新账号安全信息，并同步 users 表认证字段。
// user_security.user_id 关联 users.user_id，因此先锁定 users 行再写入安全表，保证多表数据一致。
func (r *UserRepo) UpdateSecurityByUserID(
	ctx context.Context,
	userID string,
	d *UserDtoPackage.HGUpdateUserSecurityReqDTO,
	passwordHash *string,
	salt *string,
) error {
	queryCtx, cancel := context.WithTimeout(ctx, userRepoQueryTimeout)
	defer cancel()

	// 开启事务
	tx, err := r.BeginTx(queryCtx, nil)
	if err != nil {
		return wrapUserRepoWriteErr("begin user security tx", err)
	}
	defer tx.Rollback()

	// 锁住users表中要修改的那行数据
	security, err := getSecurityBaseForUpdate(queryCtx, tx, userID)
	if err != nil {
		return err
	}
	applySecurityValues(security, d, passwordHash, salt)

	if err = updateUsersAuthFields(queryCtx, tx, userID, d, passwordHash, salt); err != nil {
		return err
	}

	securityID, err := getUserSecurityIDForUpdate(queryCtx, tx, security.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		if err = insertUserSecurity(queryCtx, tx, security); err != nil {
			if isDuplicateUserSecurityUserID(err) {
				if retryErr := updateUserSecurity(queryCtx, tx, security.UserID, d, passwordHash, salt); retryErr != nil {
					return retryErr
				}
			} else {
				return err
			}
		}
	} else if securityID > 0 {
		if err = updateUserSecurity(queryCtx, tx, security.UserID, d, passwordHash, salt); err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return wrapUserRepoWriteErr("commit user security tx", err)
	}

	return nil
}

// GetSecurityByUserID 按业务 user_id 读取 user_security 表完整字段。
func (r *UserRepo) GetSecurityByUserID(ctx context.Context, userID string) (*UserModelsPackage.HGUserSecurityModel, error) {
	queryCtx, cancel := context.WithTimeout(ctx, userRepoQueryTimeout)
	defer cancel()

	security := &UserModelsPackage.HGUserSecurityModel{}
	err := r.QueryRow(
		queryCtx,
		SQLQueriesPackage.SelectUserSecurityByUserIDSQL,
		userID,
	).Scan(
		&security.ID,
		&security.UserID,
		&security.Email,
		&security.Phone,
		&security.PasswordHash,
		&security.Salt,
		&security.QQ,
		&security.Wechat,
		&security.CreatedAt,
		&security.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return security, nil
}

// getSecurityBaseForUpdate 锁定 users 行，并取 user_security 插入所需的默认认证字段。
func getSecurityBaseForUpdate(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
) (*UserModelsPackage.HGUserSecurityModel, error) {
	security := &UserModelsPackage.HGUserSecurityModel{}
	err := tx.QueryRowContext(
		ctx,
		SQLQueriesPackage.SelectUserSecurityBaseForUpdateSQL,
		userID,
	).Scan(
		&security.UserID,
		&security.Email,
		&security.Phone,
		&security.PasswordHash,
		&security.Salt,
	)
	if err != nil {
		return nil, err
	}

	return security, nil
}

// updateUsersAuthFields 同步 users 表中仍被登录链路使用的邮箱、手机和密码字段。
func updateUsersAuthFields(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	d *UserDtoPackage.HGUpdateUserSecurityReqDTO,
	passwordHash *string,
	salt *string,
) error {
	setClauses := make([]string, 0, 4)
	args := make([]any, 0, 5)

	if d.Email != nil {
		setClauses = append(setClauses, "`email` = ?")
		args = append(args, *d.Email)
	}
	if d.Phone != nil {
		setClauses = append(setClauses, "`phone` = ?")
		args = append(args, *d.Phone)
	}
	if passwordHash != nil && salt != nil {
		setClauses = append(setClauses, "`password_hash` = ?", "`salt` = ?")
		args = append(args, *passwordHash, *salt)
	}
	if len(setClauses) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE users SET %s WHERE user_id = ?", strings.Join(setClauses, ", "))
	args = append(args, userID)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return wrapUserSecurityWriteErr("update users auth fields", err)
	}

	return nil
}

// getUserSecurityIDForUpdate 查询并锁定当前用户的安全记录，未创建时返回 sql.ErrNoRows。
func getUserSecurityIDForUpdate(ctx context.Context, tx *sql.Tx, userID string) (int64, error) {
	var securityID int64
	err := tx.QueryRowContext(
		ctx,
		SQLQueriesPackage.SelectUserSecurityIDForUpdateSQL,
		userID,
	).Scan(&securityID)

	return securityID, err
}

// insertUserSecurity 创建用户安全记录，未修改的邮箱、手机和密码默认沿用 users 当前值。
func insertUserSecurity(ctx context.Context, tx *sql.Tx, security *UserModelsPackage.HGUserSecurityModel) error {
	_, err := tx.ExecContext(
		ctx,
		SQLQueriesPackage.InsertUserSecuritySQL,
		security.UserID,
		security.Email,
		security.Phone,
		security.PasswordHash,
		security.Salt,
		security.QQ,
		security.Wechat,
	)
	if err != nil {
		return wrapUserSecurityWriteErr("insert user security", err)
	}

	return nil
}

// updateUserSecurity 动态更新已有安全记录，只覆盖请求中显式传入的字段。
func updateUserSecurity(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	d *UserDtoPackage.HGUpdateUserSecurityReqDTO,
	passwordHash *string,
	salt *string,
) error {
	setClauses := make([]string, 0, 6)
	args := make([]any, 0, 7)

	if d.Email != nil {
		setClauses = append(setClauses, "`email` = ?")
		args = append(args, *d.Email)
	}
	if d.Phone != nil {
		setClauses = append(setClauses, "`phone` = ?")
		args = append(args, *d.Phone)
	}
	if passwordHash != nil && salt != nil {
		setClauses = append(setClauses, "`password_hash` = ?", "`salt` = ?")
		args = append(args, *passwordHash, *salt)
	}
	if d.QQ != nil {
		setClauses = append(setClauses, "`qq` = ?")
		args = append(args, *d.QQ)
	}
	if d.Wechat != nil {
		setClauses = append(setClauses, "`wechat` = ?")
		args = append(args, *d.Wechat)
	}
	if len(setClauses) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE user_security SET %s WHERE user_id = ?", strings.Join(setClauses, ", "))
	args = append(args, userID)
	//执行无返回结果的 SQL 语句
	// 适合：UPDATE / INSERT / DELETE / CREATE TABLE 这类不返回多行数据的写 SQL；
	// 不适合 SELECT（查数据要用 QueryContext）
	// 在事务tx中执行UPDATE，带ctx超时控制
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		// 执行失败，包装错误返回
		return wrapUserSecurityWriteErr("update user security", err)
	}

	return nil
}

// applySecurityValues 将请求字段应用到插入模型，保证新建 user_security 时字段完整。
func applySecurityValues(
	security *UserModelsPackage.HGUserSecurityModel,
	d *UserDtoPackage.HGUpdateUserSecurityReqDTO,
	passwordHash *string,
	salt *string,
) {
	if d.Email != nil {
		security.Email = sql.NullString{String: *d.Email, Valid: true}
	}
	if d.Phone != nil {
		security.Phone = sql.NullString{String: *d.Phone, Valid: true}
	}
	if passwordHash != nil && salt != nil {
		security.PasswordHash = sql.NullString{String: *passwordHash, Valid: true}
		security.Salt = sql.NullString{String: *salt, Valid: true}
	}
	if d.QQ != nil {
		security.QQ = sql.NullString{String: *d.QQ, Valid: true}
	}
	if d.Wechat != nil {
		security.Wechat = sql.NullString{String: *d.Wechat, Valid: true}
	}
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

// wrapUserSecurityWriteErr 将唯一键冲突转换成稳定业务错误，避免泄露底层 SQL 细节到 service。
func wrapUserSecurityWriteErr(operation string, err error) error {
	if isMySQLDuplicateKey(err) {
		return fmt.Errorf("%w: %w", ErrUserSecurityDuplicate, err)
	}

	return wrapUserRepoWriteErr(operation, err)
}

// isMySQLDuplicateKey 判断 MySQL 唯一键冲突。
func isMySQLDuplicateKey(err error) bool {
	// 定义 MySQL 驱动专属错误类型变量，用来承接数据库原生错误
	var mysqlErr *mysql.MySQLError
	// errors.As：把原始错误 err 尝试转换成 mysql.MySQLError（MySQL 驱动专属错误）；转换成功返回 true，失败说明不是 MySQL 数据库错误
	// 转换成功后，判断 MySQL 错误码是否等于1062
	// 1062 = ER_DUP_ENTRY：唯一索引 / 主键重复，插入重复数据
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

// isDuplicateUserSecurityUserID 判断 user_security.user_id 唯一键冲突，用于并发插入兜底重试更新。
func isDuplicateUserSecurityUserID(err error) bool {
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
		return false
	}

	return strings.Contains(mysqlErr.Message, "uk_user_security_user_id")
}

package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

// HGRepository 在 MySQL 上执行 IAM 点查、游标读取和短事务写入。
type HGRepository struct {
	db *sql.DB // 应用共享数据库连接池；不由本模块关闭。
}

// hgRoleRow 仅承载 iam_roles 的 SQL 扫描结果，不作为 API 协议。
type hgRoleRow struct {
	id          uint64    // 数据库角色主键。
	code        string    // 数据库角色编码。
	name        string    // 数据库角色名称。
	description string    // 数据库角色描述。
	status      int       // 数据库启停状态。
	builtIn     bool      // 数据库内置角色标记。
	createdAt   time.Time // 数据库创建时间。
}

// hgBindingRow 仅承载 iam_user_role_bindings 的联查结果。
type hgBindingRow struct {
	id        uint64       // 数据库绑定主键。
	userID    string       // 业务用户标识。
	roleID    uint64       // 数据库角色主键。
	roleCode  string       // 联查的角色编码。
	scopeType string       // 数据库作用域类型。
	scopeID   string       // 数据库作用域标识。
	expiresAt sql.NullTime // 可空失效时间。
}

// NewHGRepository 创建 IAM 存储适配器。
func NewHGRepository(db *sql.DB) *HGRepository { return &HGRepository{db: db} }

func (r *HGRepository) hgActiveAdmin(ctx context.Context, userID string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMManager, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return one == 1, err
}

func (r *HGRepository) hgHasPermission(ctx context.Context, userID, permission, scopeType, scopeID string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMPermission, userID, scopeType, scopeID, permission).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return one == 1, err
}

func hgScanRole(row interface{ Scan(...any) error }) (HGRole, error) {
	var dbRole hgRoleRow
	err := row.Scan(&dbRole.id, &dbRole.code, &dbRole.name, &dbRole.description, &dbRole.status, &dbRole.builtIn, &dbRole.createdAt)
	if err != nil {
		return HGRole{}, err
	}
	return HGRole{ID: dbRole.id, Code: dbRole.code, Name: dbRole.name, Description: dbRole.description, Status: dbRole.status, BuiltIn: dbRole.builtIn, CreatedAt: dbRole.createdAt}, nil
}

func (r *HGRepository) hgRole(ctx context.Context, id uint64) (HGRole, error) {
	role, err := hgScanRole(r.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMRole, id))
	if errors.Is(err, sql.ErrNoRows) {
		return HGRole{}, ErrHGNotFound
	}
	return role, err
}

func (r *HGRepository) hgListRoles(ctx context.Context, cursor uint64, limit int) ([]HGRole, bool, error) {
	query := SQLQueriesPackage.HGSelectIAMRolesFirst
	args := []any{limit + 1}
	if cursor > 0 {
		query, args = SQLQueriesPackage.HGSelectIAMRolesAfter, []any{cursor, limit + 1}
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	items := make([]HGRole, 0, limit)
	for rows.Next() {
		role, scanErr := hgScanRole(rows)
		if scanErr != nil {
			return nil, false, scanErr
		}
		items = append(items, role)
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func hgAudit(ctx context.Context, tx *sql.Tx, actor, action, targetType, targetID, requestID string, detail any) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMAudit, actor, action, targetType, targetID, requestID, data)
	return err
}

func hgDuplicate(err error) bool {
	var dbErr *mysql.MySQLError
	return errors.As(err, &dbErr) && dbErr.Number == 1062
}

func (r *HGRepository) hgCreateRole(ctx context.Context, actor, requestID, code, name, description string, permissions []string) (HGRole, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HGRole{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMRole, code, name, description)
	if hgDuplicate(err) {
		return HGRole{}, ErrHGConflict
	}
	if err != nil {
		return HGRole{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return HGRole{}, err
	}
	for _, permission := range permissions {
		result, insertErr := tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMRolePermission, id, permission)
		if insertErr != nil {
			return HGRole{}, insertErr
		}
		n, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return HGRole{}, rowsErr
		}
		if n != 1 {
			return HGRole{}, ErrHGInvalid
		}
	}
	if err = hgAudit(ctx, tx, actor, "iam.role.create", "role", strconv.FormatInt(id, 10), requestID, map[string]any{"code": code}); err != nil {
		return HGRole{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGRole{}, err
	}
	return r.hgRole(ctx, uint64(id))
}

func (r *HGRepository) hgUpdateRole(ctx context.Context, actor, requestID string, id uint64, name, description string) (HGRole, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HGRole{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, SQLQueriesPackage.HGUpdateIAMRole, name, description, id)
	if err != nil {
		return HGRole{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return HGRole{}, err
	}
	if n == 0 {
		return HGRole{}, ErrHGConflict
	}
	if err = hgAudit(ctx, tx, actor, "iam.role.update", "role", strconv.FormatUint(id, 10), requestID, map[string]any{"name": name}); err != nil {
		return HGRole{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGRole{}, err
	}
	return r.hgRole(ctx, id)
}

func (r *HGRepository) hgDisableRole(ctx context.Context, actor, requestID string, id uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, SQLQueriesPackage.HGDisableIAMRole, id, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrHGConflict
	}
	if err = hgAudit(ctx, tx, actor, "iam.role.disable", "role", strconv.FormatUint(id, 10), requestID, map[string]any{}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *HGRepository) hgBindings(ctx context.Context, userID string, cursor uint64, limit int) ([]HGBinding, bool, error) {
	query := SQLQueriesPackage.HGSelectIAMBindingsFirst
	args := []any{userID, limit + 1}
	if cursor > 0 {
		query, args = SQLQueriesPackage.HGSelectIAMBindingsAfter, []any{userID, cursor, limit + 1}
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	items := make([]HGBinding, 0)
	for rows.Next() {
		var row hgBindingRow
		if err = rows.Scan(&row.id, &row.userID, &row.roleID, &row.roleCode, &row.scopeType, &row.scopeID, &row.expiresAt); err != nil {
			return nil, false, err
		}
		item := HGBinding{ID: row.id, UserID: row.userID, RoleID: row.roleID, RoleCode: row.roleCode, ScopeType: row.scopeType, ScopeID: row.scopeID}
		if row.expiresAt.Valid {
			item.ExpiresAt = &row.expiresAt.Time
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

func (r *HGRepository) hgGrant(ctx context.Context, actor, requestID, userID string, roleID uint64, scopeType, scopeID string, expires *time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var roleCode string
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMActiveRoleCode, roleID).Scan(&roleCode)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGNotFound
	}
	if err != nil {
		return err
	}
	// 角色行锁与权限替换使用相同锁顺序，授权前复核权限集合，避免读取后发生扩权。
	permissionRows, err := tx.QueryContext(ctx, SQLQueriesPackage.HGSelectIAMRolePermissions, roleID, 201)
	if err != nil {
		return err
	}
	permissionCodes := make([]string, 0)
	for permissionRows.Next() {
		var code string
		if err = permissionRows.Scan(&code); err != nil {
			permissionRows.Close()
			return err
		}
		permissionCodes = append(permissionCodes, code)
	}
	err = permissionRows.Err()
	permissionRows.Close()
	if err != nil {
		return err
	}
	if len(permissionCodes) > 200 {
		return ErrHGForbidden
	}
	for _, code := range permissionCodes {
		var one int
		err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMPermission, actor, "global", "", code).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		if err != nil {
			return err
		}
		if one != 1 {
			return ErrHGForbidden
		}
	}
	var existingID uint64
	var status int
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMBinding, userID, roleID, scopeType, scopeID).Scan(&existingID, &status)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		if status == 1 {
			return ErrHGConflict
		}
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.HGReactivateIAMBinding, expires, actor, existingID)
	} else {
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMBinding, userID, roleID, scopeType, scopeID, expires, actor)
	}
	if hgDuplicate(err) {
		return ErrHGConflict
	}
	if err != nil {
		return err
	}
	if err = hgAudit(ctx, tx, actor, "iam.binding.grant", "user", userID, requestID, map[string]any{"roleCode": roleCode, "scopeType": scopeType, "scopeId": scopeID}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *HGRepository) hgRevoke(ctx context.Context, actor, requestID, userID string, bindingID uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var roleCode string
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMBindingRoleForRevoke, bindingID, userID).Scan(&roleCode)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGNotFound
	}
	if err != nil {
		return err
	}
	if roleCode == "super_admin" {
		return ErrHGForbidden
	}
	_, err = tx.ExecContext(ctx, SQLQueriesPackage.HGRevokeIAMBinding, bindingID, userID)
	if err != nil {
		return err
	}
	if err = hgAudit(ctx, tx, actor, "iam.binding.revoke", "user", userID, requestID, map[string]any{"bindingId": bindingID, "roleCode": roleCode}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *HGRepository) hgOrganization(ctx context.Context, scopeType, scopeID string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMOrganization, scopeID, scopeType).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return one == 1, err
}

func (r *HGRepository) hgPermissions(ctx context.Context) ([]map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.HGSelectIAMPermissions, 201)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]string, 0)
	for rows.Next() {
		var code, description string
		if err = rows.Scan(&code, &description); err != nil {
			return nil, err
		}
		items = append(items, map[string]string{"code": code, "description": description})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(items) > 200 {
		return nil, ErrHGConflict
	}
	return items, nil
}

func (r *HGRepository) hgRolePermissions(ctx context.Context, roleID uint64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.HGSelectIAMRolePermissions, roleID, 201)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0)
	for rows.Next() {
		var code string
		if err = rows.Scan(&code); err != nil {
			return nil, err
		}
		items = append(items, code)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(items) > 200 {
		return nil, ErrHGForbidden
	}
	return items, nil
}

func (r *HGRepository) hgSetRolePermissions(ctx context.Context, actor, requestID string, roleID uint64, codes []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var builtIn bool
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMRoleForPermissionWrite, roleID).Scan(&builtIn)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGNotFound
	}
	if err != nil {
		return err
	}
	if builtIn {
		return ErrHGForbidden
	}
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGDeleteIAMRolePermissions, roleID); err != nil {
		return err
	}
	for _, code := range codes {
		res, insertErr := tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMRolePermission, roleID, code)
		if insertErr != nil {
			return insertErr
		}
		n, rowsErr := res.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if n != 1 {
			return fmt.Errorf("%w: 未知权限编码", ErrHGInvalid)
		}
	}
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGBumpIAMRoleVersion, roleID); err != nil {
		return err
	}
	if err = hgAudit(ctx, tx, actor, "iam.role.permissions", "role", strconv.FormatUint(roleID, 10), requestID, map[string]any{"permissions": codes}); err != nil {
		return err
	}
	return tx.Commit()
}

package OpsRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Repository 定义运维管理数据访问接口
type Repository struct {
	db                 *sql.DB
	adminEmailOnce     sync.Once
	adminEmailExists   bool
	adminEmailCheckErr error
}

// NewRepository 创建运维管理数据访问实例
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateRole 创建角色
func (r *Repository) CreateRole(ctx context.Context, name, description string) (string, error) {
	res, err := r.db.ExecContext(ctx, SQLQueriesPackage.InsertOpsRoleSQL, name, description)
	if err != nil {
		return "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

// GetRoleList 获取角色列表。
// 千万级表约束：
// - 使用 idx_status_id(status,id) 复合索引，查询条件固定为 status=1，并按 id 倒序做游标翻页。
// - cursor=0 表示第一页；cursor>0 时查询 id < cursor，避免 OFFSET 深分页扫描和回表丢弃大量行。
// - 不执行 COUNT(*)，Total 返回 -1 表示大表场景不做实时总数统计，避免统计锁竞争和 Buffer Pool 压力。
// - 每次最多取 pageSize+1 条判断 hasMore，业务返回仍限制为 pageSize 条。
func (r *Repository) GetRoleList(ctx context.Context, cursor int64, pageSize int) ([]map[string]interface{}, int64, bool, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	queryLimit := pageSize + 1
	querySQL := SQLQueriesPackage.SelectOpsRoleListFirstSQL
	args := []interface{}{queryLimit}
	if cursor > 0 {
		querySQL = SQLQueriesPackage.SelectOpsRoleListByCursorSQL
		args = []interface{}{cursor, queryLimit}
	}

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()

	list := make([]map[string]interface{}, 0, queryLimit)
	for rows.Next() {
		var id int64
		var name, description string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &description, &createdAt); err != nil {
			return nil, 0, false, err
		}
		list = append(list, map[string]interface{}{
			"id":          strconv.FormatInt(id, 10),
			"idInt":       id,
			"name":        name,
			"description": description,
			"createdAt":   createdAt.Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}
	hasMore := len(list) > pageSize
	if hasMore {
		list = list[:pageSize]
	}
	return list, -1, hasMore, nil
}

// GetAdminUserList 获取管理员列表。
// 千万级表约束：使用 admin_user.id 倒序 cursor 分页，cursor=0 表示首页，cursor>0 查询 id<cursor。
// 不执行 COUNT(*) 和 OFFSET；每次多取 1 条判断 hasMore。建议 admin_user 建立 (is_delete,id) 复合索引支撑软删除过滤和稳定排序。
func (r *Repository) GetAdminUserList(ctx context.Context, cursor int64, pageSize int) ([]map[string]interface{}, int64, bool, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	queryLimit := pageSize + 1
	hasEmail := r.hasAdminUserEmailColumn(ctx)
	querySQL := SQLQueriesPackage.SelectOpsAdminUserListFirstWithoutEmailSQL
	args := []interface{}{queryLimit}
	if cursor > 0 {
		querySQL = SQLQueriesPackage.SelectOpsAdminUserListByCursorWithoutEmailSQL
		args = []interface{}{cursor, queryLimit}
	}
	if hasEmail {
		querySQL = SQLQueriesPackage.SelectOpsAdminUserListFirstWithEmailSQL
		if cursor > 0 {
			querySQL = SQLQueriesPackage.SelectOpsAdminUserListByCursorWithEmailSQL
		}
	}

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()

	list := make([]map[string]interface{}, 0, queryLimit)
	for rows.Next() {
		var id int64
		var name, nickName, mobile string
		var email sql.NullString
		var status int
		if hasEmail {
			if err := rows.Scan(&id, &name, &nickName, &email, &mobile, &status); err != nil {
				return nil, 0, false, err
			}
		} else {
			if err := rows.Scan(&id, &name, &nickName, &mobile, &status); err != nil {
				return nil, 0, false, err
			}
		}
		list = append(list, map[string]interface{}{
			"id":       strconv.FormatInt(id, 10),
			"name":     name,
			"nickName": nickName,
			"email":    email.String,
			"mobile":   mobile,
			"status":   status,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}
	hasMore := len(list) > pageSize
	if hasMore {
		list = list[:pageSize]
	}
	return list, -1, hasMore, nil
}

// SearchAdminUsers 按管理员 ID、姓名、昵称、邮箱、手机号搜索管理员。
// 千万级表约束：
// - id 使用主键等值查询；mobile 使用唯一索引 idx_mobile 的前缀 LIKE；name/nick_name 使用 idx_name/idx_nick_name 的前缀 LIKE。
// - email 是新迁移字段：当前库可能尚未执行加列迁移，所以先检测列是否存在；存在时才纳入 idx_email 前缀搜索和 SELECT 字段。
// - 不支持 "%keyword%" 包含查询，避免 BTree 索引失效导致全表扫描。
// - 强制 limit 最大 20，且不额外执行 COUNT(*)，避免后台搜索接口在高并发下返回大结果集或触发大范围统计。
func (r *Repository) SearchAdminUsers(ctx context.Context, keyword string, limit int) ([]map[string]interface{}, int64, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []map[string]interface{}{}, 0, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	conditions := []string{SQLQueriesPackage.OpsAdminUserActiveConditionSQL}
	args := make([]interface{}, 0, 5)
	hasEmail := r.hasAdminUserEmailColumn(ctx)
	likePrefix := keyword + "%"
	if id, err := strconv.ParseInt(keyword, 10, 64); err == nil && id > 0 {
		conditions = append(conditions, SQLQueriesPackage.OpsAdminUserIDConditionSQL)
		args = append(args, id)
	} else if hasEmail {
		conditions = append(conditions, SQLQueriesPackage.OpsAdminUserKeywordWithEmailConditionSQL)
		args = append(args, likePrefix, likePrefix, likePrefix, likePrefix)
	} else {
		conditions = append(conditions, SQLQueriesPackage.OpsAdminUserKeywordWithoutEmailConditionSQL)
		args = append(args, likePrefix, likePrefix, likePrefix)
	}

	whereSQL := strings.Join(conditions, " AND ")
	selectSQL := SQLQueriesPackage.SelectOpsAdminUserWithoutEmailSQL
	if hasEmail {
		selectSQL = SQLQueriesPackage.SelectOpsAdminUserWithEmailSQL
	}
	querySQL := fmt.Sprintf("%s WHERE %s ORDER BY `id` DESC LIMIT ?", selectSQL, whereSQL)
	queryArgs := append(args, limit)
	rows, err := r.db.QueryContext(ctx, querySQL, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]map[string]interface{}, 0, limit)
	for rows.Next() {
		var id int64
		var name, nickName, mobile string
		var email sql.NullString
		var status int
		if hasEmail {
			if err := rows.Scan(&id, &name, &nickName, &email, &mobile, &status); err != nil {
				return nil, 0, err
			}
		} else {
			if err := rows.Scan(&id, &name, &nickName, &mobile, &status); err != nil {
				return nil, 0, err
			}
		}
		list = append(list, map[string]interface{}{
			"id":       strconv.FormatInt(id, 10),
			"name":     name,
			"nickName": nickName,
			"email":    email.String,
			"mobile":   mobile,
			"status":   status,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, int64(len(list)), nil
}

// SearchAdminCandidates 搜索可添加为管理员的注册用户候选。
// 千万级表约束：避免多字段 OR；按 users.id 主键和 user_id/user_name/email/phone 唯一索引分别小范围查询，再在内存合并去重。
func (r *Repository) SearchAdminCandidates(ctx context.Context, keyword string, limit int) ([]map[string]interface{}, int64, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []map[string]interface{}{}, 0, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	list := make([]map[string]interface{}, 0, limit)
	seen := make(map[string]struct{}, limit)
	appendRows := func(rows *sql.Rows) error {
		defer rows.Close()
		for rows.Next() {
			item, err := scanAdminCandidateRow(rows)
			if err != nil {
				return err
			}
			id := item["id"].(string)
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			list = append(list, item)
			if len(list) >= limit {
				break
			}
		}
		return rows.Err()
	}

	if id, err := strconv.ParseInt(keyword, 10, 64); err == nil && id > 0 {
		rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectOpsAdminCandidateByIDSQL, id, limit)
		if err != nil {
			return nil, 0, err
		}
		if err := appendRows(rows); err != nil {
			return nil, 0, err
		}
	}
	if len(list) >= limit {
		return list, int64(len(list)), nil
	}

	likePrefix := keyword + "%"
	queries := []string{
		SQLQueriesPackage.SelectOpsAdminCandidateByUserIDPrefixSQL,
		SQLQueriesPackage.SelectOpsAdminCandidateByUserNamePrefixSQL,
		SQLQueriesPackage.SelectOpsAdminCandidateByEmailPrefixSQL,
		SQLQueriesPackage.SelectOpsAdminCandidateByPhonePrefixSQL,
	}
	for _, querySQL := range queries {
		if len(list) >= limit {
			break
		}
		rows, err := r.db.QueryContext(ctx, querySQL, likePrefix, limit-len(list))
		if err != nil {
			return nil, 0, err
		}
		if err := appendRows(rows); err != nil {
			return nil, 0, err
		}
	}
	return list, int64(len(list)), nil
}

func scanAdminCandidateRow(rows *sql.Rows) (map[string]interface{}, error) {
	var id int64
	var userID, userName, nickName, email, phone sql.NullString
	if err := rows.Scan(&id, &userID, &userName, &nickName, &email, &phone); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":       strconv.FormatInt(id, 10),
		"userId":   userID.String,
		"userName": userName.String,
		"nickName": nickName.String,
		"email":    email.String,
		"phone":    phone.String,
	}, nil
}

// AddAdminFromUser 将注册用户添加为管理员。
// 写入路径使用 users.id 主键定位候选用户，admin_user 的 email/mobile 唯一索引负责并发重复提交保护。
func (r *Repository) AddAdminFromUser(ctx context.Context, operatorID, userID string) (map[string]interface{}, error) {
	userIDInt, err := strconv.ParseInt(strings.TrimSpace(userID), 10, 64)
	if err != nil || userIDInt <= 0 {
		return nil, fmt.Errorf("invalid userID")
	}
	operatorIDInt, _ := strconv.ParseInt(strings.TrimSpace(operatorID), 10, 64)

	insertSQL := SQLQueriesPackage.InsertOpsAdminFromUserWithoutEmailSQL
	if r.hasAdminUserEmailColumn(ctx) {
		insertSQL = SQLQueriesPackage.InsertOpsAdminFromUserWithEmailSQL
	}

	res, err := r.db.ExecContext(ctx, insertSQL, operatorIDInt, operatorIDInt, userIDInt)
	if err != nil {
		if isDuplicateKeyError(err) {
			return r.getAdminByUserPhone(ctx, userIDInt)
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, fmt.Errorf("用户不存在")
	}
	return r.getAdminByID(ctx, id)
}

func (r *Repository) getAdminByUserPhone(ctx context.Context, userID int64) (map[string]interface{}, error) {
	var mobile string
	if err := r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectOpsUserPhoneByIDSQL, userID).Scan(&mobile); err != nil {
		return nil, err
	}
	return r.getAdminByMobile(ctx, mobile)
}

func (r *Repository) getAdminByMobile(ctx context.Context, mobile string) (map[string]interface{}, error) {
	querySQL := SQLQueriesPackage.SelectOpsAdminByMobileWithoutEmailSQL
	if r.hasAdminUserEmailColumn(ctx) {
		querySQL = SQLQueriesPackage.SelectOpsAdminByMobileWithEmailSQL
	}
	return r.scanAdminRow(r.db.QueryRowContext(ctx, querySQL, mobile), r.hasAdminUserEmailColumn(ctx))
}

func (r *Repository) getAdminByID(ctx context.Context, adminID int64) (map[string]interface{}, error) {
	hasEmail := r.hasAdminUserEmailColumn(ctx)
	querySQL := SQLQueriesPackage.SelectOpsAdminByIDWithoutEmailSQL
	if hasEmail {
		querySQL = SQLQueriesPackage.SelectOpsAdminByIDWithEmailSQL
	}
	return r.scanAdminRow(r.db.QueryRowContext(ctx, querySQL, adminID), hasEmail)
}

func (r *Repository) scanAdminRow(row *sql.Row, hasEmail bool) (map[string]interface{}, error) {
	var id int64
	var name, nickName, mobileValue string
	var email sql.NullString
	var status int
	if hasEmail {
		if err := row.Scan(&id, &name, &nickName, &email, &mobileValue, &status); err != nil {
			return nil, err
		}
	} else {
		if err := row.Scan(&id, &name, &nickName, &mobileValue, &status); err != nil {
			return nil, err
		}
	}
	return map[string]interface{}{
		"id":       strconv.FormatInt(id, 10),
		"name":     name,
		"nickName": nickName,
		"email":    email.String,
		"mobile":   mobileValue,
		"status":   status,
	}, nil
}

func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysql.MySQLError
	return err != nil && (errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 || strings.Contains(err.Error(), "Duplicate entry"))
}

func (r *Repository) hasAdminUserEmailColumn(ctx context.Context) bool {
	r.adminEmailOnce.Do(func() {
		var exists int
		// 这里查询的是 MySQL 的元数据表 INFORMATION_SCHEMA.COLUMNS，不是扫描业务表 admin_user。
		// 作用：判断当前连接的数据库中，admin_user 表是否已经存在 email 字段。
		// SQL 含义：
		// - INFORMATION_SCHEMA.COLUMNS：MySQL 内置的数据字典表，记录每个库、每张表、每个字段的结构信息。
		// - TABLE_SCHEMA = DATABASE()：只检查当前连接正在使用的数据库，避免同一 MySQL 实例里其他库的同名表干扰判断。
		// - TABLE_NAME = 'admin_user'：只检查运营管理员表。
		// - COLUMN_NAME = 'email'：只判断 email 字段是否存在。
		// - COUNT(*)：字段存在时返回 1，不存在时返回 0。
		//
		// 为什么要这样做：email 是后续补充的迁移字段，部分环境可能还没执行 ALTER TABLE。
		// 如果直接 SELECT `email` 或 WHERE `email` LIKE ?，旧库会报 MySQL 1054 Unknown column。
		// 先检测字段是否存在，再动态决定 SearchAdminUsers 是否拼接 email 查询，可以让旧库继续按 id/name/nick_name/mobile 搜索。
		//
		// 千万级表约束：这个检查只访问系统元数据，不读取 admin_user 业务数据；并且用 sync.Once 缓存结果，
		// 每个 Repository 实例最多执行一次，避免高并发搜索时反复访问 INFORMATION_SCHEMA。
		// 注意：如果服务运行中手工新增 email 字段，需要重启服务后 sync.Once 缓存才会刷新。
		r.adminEmailCheckErr = r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectOpsTableColumnExistsSQL, "admin_user", "email").Scan(&exists)
		r.adminEmailExists = r.adminEmailCheckErr == nil && exists > 0
	})
	return r.adminEmailExists
}

// AssignUserRoles 分配用户角色
func (r *Repository) AssignUserRoles(ctx context.Context, userID string, roleIDs []string) error {
	adminUserID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || adminUserID <= 0 {
		return fmt.Errorf("invalid userID")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 千万级关联表约束：admin_user_role 通过唯一索引 (admin_user_id, role_id) 命中指定管理员；
	// 单个管理员角色数很小，采用同一事务内先删后批量插入，保证提交后关联集合完整替换。
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.DeleteOpsAdminUserRolesSQL, adminUserID); err != nil {
		return err
	}

	if len(roleIDs) == 0 {
		return tx.Commit()
	}

	stmt, err := tx.PrepareContext(ctx, SQLQueriesPackage.InsertOpsAdminUserRoleSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	seen := make(map[int64]struct{}, len(roleIDs))
	for _, roleIDText := range roleIDs {
		roleID, err := strconv.ParseInt(roleIDText, 10, 64)
		if err != nil || roleID <= 0 {
			return fmt.Errorf("invalid roleID")
		}
		if _, ok := seen[roleID]; ok {
			continue
		}
		seen[roleID] = struct{}{}
		if _, err := stmt.ExecContext(ctx, adminUserID, roleID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetUserRoles 获取用户角色
func (r *Repository) GetUserRoles(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	adminUserID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || adminUserID <= 0 {
		return nil, fmt.Errorf("invalid userID")
	}

	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectOpsUserRolesSQL, adminUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]map[string]interface{}, 0, 8)
	for rows.Next() {
		var id int64
		var name, description string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &description, &createdAt); err != nil {
			return nil, err
		}
		list = append(list, map[string]interface{}{
			"id":          strconv.FormatInt(id, 10),
			"name":        name,
			"description": description,
			"createdAt":   createdAt.Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// CreateMenu 创建菜单
func (r *Repository) CreateMenu(ctx context.Context, name, path, parentID string, sort int, icon string) (string, error) {
	// TODO: 实现创建菜单逻辑
	return "", nil
}

// GetMenuList 获取菜单列表
func (r *Repository) GetMenuList(ctx context.Context) ([]map[string]interface{}, error) {
	// TODO: 实现获取菜单列表逻辑
	return nil, nil
}

// AssignRolePermissions 分配角色权限
func (r *Repository) AssignRolePermissions(ctx context.Context, roleID string, menuIDs, permissions []string) error {
	// TODO: 实现分配角色权限逻辑
	return nil
}

// GetRolePermissions 获取角色权限
func (r *Repository) GetRolePermissions(ctx context.Context, roleID string) ([]string, []string, error) {
	// TODO: 实现获取角色权限逻辑
	return nil, nil, nil
}

// CreateFile 创建文件记录
func (r *Repository) CreateFile(ctx context.Context, name string, size int64, mimeType, url string) (string, error) {
	// TODO: 实现创建文件记录逻辑
	return "", nil
}

// GetFileList 获取文件列表
func (r *Repository) GetFileList(ctx context.Context, page, pageSize int) ([]map[string]interface{}, int64, error) {
	// TODO: 实现获取文件列表逻辑
	return nil, 0, nil
}

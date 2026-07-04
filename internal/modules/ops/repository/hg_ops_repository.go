package OpsRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	UtilsPackage "MLC_GO/internal/pkg/utils"
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

type opsRoleBinding struct {
	BusinessID string
	InternalID int64
	Name       string
	Status     int
}

// NewRepository 创建运维管理数据访问实例
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateRole 创建角色
func (r *Repository) CreateRole(ctx context.Context, name, description string) (string, error) {
	roleID := UtilsPackage.GenerateRoleID()
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.InsertOpsRoleSQL, roleID, name, description)
	if err != nil {
		return "", err
	}
	return roleID, nil
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
		var roleID sql.NullString
		var id int64
		var name, description string
		var createdAt time.Time
		if err := rows.Scan(&roleID, &id, &name, &description, &createdAt); err != nil {
			return nil, 0, false, err
		}
		list = append(list, map[string]interface{}{
			"id":          roleID.String,
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

// SearchAdminUsers 按管理员 ID、邮箱、手机号、用户名等关键词搜索管理员。
// 千万级表约束：
// - 管理员 ID 使用 admin_user.user_id，也就是 000002 users.user_id；支持前缀 LIKE，满足角色分配页输入完整或部分管理员 ID 的场景。
// - mobile 使用唯一索引 idx_mobile 的前缀 LIKE；name/nick_name 使用 idx_name/idx_nick_name 的前缀 LIKE。
// - email 是新迁移字段：当前库可能尚未执行加列迁移，所以先检测列是否存在；存在时才纳入 idx_email 前缀搜索和 SELECT 字段。
// - users.user_id/user_name/email/phone 先走各自索引查 users.user_id，再按 admin_user.user_id 回表过滤软删除。
// - 支持 "%keyword%" 包含查询满足前端“输入全部或一部分”的要求，因此必须严格限制 keyword 长度和 limit。
// - 强制 limit 最大 20，且不额外执行 COUNT(*)，避免后台搜索接口在高并发下返回大结果集或触发大范围统计。
func (r *Repository) SearchAdminUsers(ctx context.Context, keyword string, limit int) ([]map[string]interface{}, int64, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []map[string]interface{}{}, 0, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	hasEmail := r.hasAdminUserEmailColumn(ctx)
	// 前端角色分配页允许输入管理员ID、用户名、邮箱或手机号的全部/一部分。
	// 因此这里使用包含匹配；limit 被严格限制到 20，避免模糊搜索一次返回过多数据。
	likePattern := "%" + keyword + "%"
	list := make([]map[string]interface{}, 0, limit)
	seen := make(map[string]struct{}, limit)
	// 多条搜索路径可能命中同一个管理员，用 seen 保证返回结果去重且不超过 limit。
	// appendAdmins 是一个匿名函数
	appendAdmins := func(rows *sql.Rows) error {
		defer rows.Close()
		for rows.Next() { // 数据库遍历， 游标向下一行移动。
			item, err := scanAdminUserRow(rows, hasEmail)
			if err != nil {
				return err
			}
			id := item["id"].(string) //类型断言其为string类型，因为item["id"]是interface{}类型，即为：any类型
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
	queryAdmins := func(condition string, args ...interface{}) error {
		if len(list) >= limit {
			return nil
		}
		whereSQL := SQLQueriesPackage.OpsAdminUserActiveConditionSQL + " AND " + condition
		selectSQL := SQLQueriesPackage.SelectOpsAdminUserWithoutEmailSQL
		if hasEmail {
			selectSQL = SQLQueriesPackage.SelectOpsAdminUserWithEmailSQL
		}
		querySQL := fmt.Sprintf("%s WHERE %s ORDER BY `id` DESC LIMIT ?", selectSQL, whereSQL)
		/* 比如：limit - len(list) = 4
		args = [status, name]
		queryArgs = [status, name, 4]
		*/
		queryArgs := append(args, limit-len(list))
		rows, err := r.db.QueryContext(ctx, querySQL, queryArgs...)
		if err != nil {
			return err
		}
		return appendAdmins(rows)
	}
	// 管理员 ID 对应 admin_user.user_id/users.user_id，是 varchar(255) 的不规则字符串；数字和非数字关键词都要支持模糊匹配。
	if err := queryAdmins(SQLQueriesPackage.OpsAdminUserIDConditionSQL, likePattern); err != nil {
		return nil, 0, err
	}
	if hasEmail {
		// 先查 admin_user 自身字段，优先返回已是管理员表直接命中的数据。
		if err := queryAdmins(SQLQueriesPackage.OpsAdminUserKeywordWithEmailConditionSQL, likePattern, likePattern, likePattern, likePattern); err != nil {
			return nil, 0, err
		}
	} else {
		// 灰度迁移兼容：admin_user.email 不存在时不引用该列，避免旧库报 Unknown column。
		if err := queryAdmins(SQLQueriesPackage.OpsAdminUserKeywordWithoutEmailConditionSQL, likePattern, likePattern, likePattern); err != nil {
			return nil, 0, err
		}
	}
	if len(list) >= limit {
		return list, int64(len(list)), nil
	}
	// ParseInt 仅判断 keyword 是否为纯数字字符串，不用于判断 users.user_id。
	// users.user_id 是 varchar(255) 的不规则字符串；数字关键词仍需继续支持手机号等字段的模糊搜索。
	if _, err := strconv.ParseInt(keyword, 10, 64); err == nil { // keyword 是纯数字字符串
		// 兼容通过 users.user_id 精确输入纯数字字符串的场景：先在 users 表按唯一索引查业务 user_id，
		// 再按 admin_user.user_id 回表，避免把 users.id 自增主键误当成管理员 ID。
		if err := r.appendAdminUsersByIDQuery(ctx, &list, seen, hasEmail, limit, SQLQueriesPackage.SelectOpsAdminUserIDByUsersIDSQL, keyword); err != nil {
			return nil, 0, err
		}
	}
	if len(list) >= limit {
		return list, int64(len(list)), nil
	}
	userIDQueries := []string{
		SQLQueriesPackage.SelectOpsAdminUserIDByUsersUserIDPrefixSQL,
		SQLQueriesPackage.SelectOpsAdminUserIDByUsersUserNamePrefixSQL,
		SQLQueriesPackage.SelectOpsAdminUserIDByUsersNickNamePrefixSQL,
		SQLQueriesPackage.SelectOpsAdminUserIDByUsersEmailPrefixSQL,
		SQLQueriesPackage.SelectOpsAdminUserIDByUsersPhonePrefixSQL,
	}
	// admin_user 未直接命中时，再按 users 表的业务 ID、用户名、昵称、邮箱、手机号补充搜索。
	// 注意：这里不再混用 000006 的 user/wechat_user/app_user.user_id，那些字段是 bigint，和 000002 users.user_id 不是同一类型。
	for _, querySQL := range userIDQueries {
		if len(list) >= limit {
			break
		}
		if err := r.appendAdminUsersByIDQuery(ctx, &list, seen, hasEmail, limit, querySQL, likePattern); err != nil {
			return nil, 0, err
		}
	}
	return list, int64(len(list)), nil
}

func scanAdminUserRow(rows *sql.Rows, hasEmail bool) (map[string]interface{}, error) {
	var id sql.NullString
	var name, nickName, mobile string
	var email sql.NullString
	var status int
	// SELECT 的第一个字段固定是 admin_user.user_id，它来自 000002 users.user_id。
	// 不要扫描 admin_user.id：该字段只是后台管理员表自增主键，不是前端角色分配页展示/传递的管理员 ID。
	// 历史管理员可能在新增 user_id 字段前创建，数据库里仍是 NULL；用 NullString 避免扫描失败，后续应通过数据回填修复。
	if hasEmail { //有邮箱
		if err := rows.Scan(&id, &name, &nickName, &email, &mobile, &status); err != nil {
			return nil, err
		}
	} else {
		// 当前数据库一行 → Go map。可以将数据库中的字段映射到 Go map。也就是将表中某一行的值映射到 Go map。
		if err := rows.Scan(&id, &name, &nickName, &mobile, &status); err != nil {
			return nil, err
		}
	}
	// TODO：优先使用结构体而不是 map[string]interface{}。例如定义一个 AdminUser 结构体，再由 JSON 序列化输出。这样类型安全、IDE 自动补全、编译期检查都会更好。
	// TODO：显式处理 sql.NullString.Valid。当前直接使用 id.String、email.String 会把数据库 NULL 和空字符串都变成 ""，如果业务上需要区分这两种情况，建议根据 Valid 返回 nil 或其他明确的值
	return map[string]interface{}{
		"id":       id.String, //数据库类型
		"name":     name,
		"nickName": nickName,
		"email":    email.String,
		"mobile":   mobile,
		"status":   status,
	}, nil
}

func (r *Repository) appendAdminUsersByIDQuery(ctx context.Context, list *[]map[string]interface{}, seen map[string]struct{}, hasEmail bool, limit int, querySQL string, keyword interface{}) error {
	rows, err := r.db.QueryContext(ctx, querySQL, keyword, limit-len(*list))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var adminUserID string
		// 候选 SQL 必须返回 users.user_id/admin_user.user_id 字符串。
		// 这样按用户名、邮箱、手机号搜索时，最终对外返回的 id 与 users.user_id 完全一致。
		if err := rows.Scan(&adminUserID); err != nil {
			return err
		}
		if len(*list) >= limit {
			break
		}
		if _, ok := seen[adminUserID]; ok {
			continue
		}
		// 候选表只负责定位 users.user_id，最终仍以 admin_user.user_id 当前状态为准，软删除或不存在的管理员直接跳过。
		item, err := r.getAdminByIDWithEmailFlag(ctx, adminUserID, hasEmail)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return err
		}
		seen[adminUserID] = struct{}{}
		*list = append(*list, item)
	}
	return rows.Err()
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
	querySQL := SQLQueriesPackage.SelectOpsAdminByPrimaryIDWithoutEmailSQL
	if hasEmail {
		querySQL = SQLQueriesPackage.SelectOpsAdminByPrimaryIDWithEmailSQL
	}
	return r.scanAdminRow(r.db.QueryRowContext(ctx, querySQL, adminID), hasEmail)
}

func (r *Repository) getAdminByIDWithEmailFlag(ctx context.Context, adminUserID string, hasEmail bool) (map[string]interface{}, error) {
	querySQL := SQLQueriesPackage.SelectOpsAdminByIDWithoutEmailSQL
	if hasEmail {
		querySQL = SQLQueriesPackage.SelectOpsAdminByIDWithEmailSQL
	}
	return r.scanAdminRow(r.db.QueryRowContext(ctx, querySQL, adminUserID), hasEmail)
}

func (r *Repository) scanAdminRow(row *sql.Row, hasEmail bool) (map[string]interface{}, error) {
	var id sql.NullString
	var name, nickName, mobileValue string
	var email sql.NullString
	var status int
	// 所有 SearchAdminUsers 相关 SELECT 的第一个字段都必须是 admin_user.user_id。
	// 该字段与 users.user_id 类型一致，前端角色分配页使用它作为管理员 ID；admin_user.id 仅保留给内部表关联。
	// 使用 NullString 是为了兼容已上线历史数据，避免 user_id 尚未回填时接口直接扫描报错。
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
		"id":       id.String,
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
	adminUserID, err := r.getAdminInternalIDByUserID(ctx, userID)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 千万级关联表约束：admin_user_role 通过唯一索引 (admin_user_id, role_id) 命中指定管理员；
	// 单个管理员角色数很小，采用同一事务内先删后批量插入，保证提交后关联集合完整替换。
	/* 在事务（Tx）上下文中执行一条 SQL 语句（通常是 INSERT / UPDATE / DELETE），并返回执行结果。
	适用于：INSERT、UPDATE、DELETE、DDL（CREATE / ALTER / DROP）
	不适用于：SELECT（查询用 QueryContext）
	*/
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.DeleteOpsAdminUserRolesSQL, adminUserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.DeleteOpsUserRoleViewSQL, adminUserID); err != nil {
		return err
	}

	if len(roleIDs) == 0 {
		// 不执行tx.Commit()，提交事务，数据不会落库
		return tx.Commit()
	}

	roleBindings, err := r.resolveRoleBindings(ctx, tx, roleIDs)
	if err != nil {
		return err
	}
	if len(roleBindings) == 0 {
		return tx.Commit()
	}

	if err := insertAdminUserRolesBatch(ctx, tx, adminUserID, roleBindings); err != nil {
		return err
	}
	if err := insertUserRoleViewBatch(ctx, tx, adminUserID, roleBindings); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *Repository) resolveRoleBindings(ctx context.Context, tx *sql.Tx, roleIDs []string) ([]opsRoleBinding, error) {
	seen := make(map[string]struct{}, len(roleIDs))
	businessRoleIDs := make([]string, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			return nil, fmt.Errorf("invalid roleID")
		}
		if _, ok := seen[roleID]; ok {
			continue
		}
		seen[roleID] = struct{}{}
		businessRoleIDs = append(businessRoleIDs, roleID)
	}
	if len(businessRoleIDs) == 0 {
		return nil, nil
	}

	// strings.Repeat，比如传入("?,", 3),结果是："?,?,?,";每一个 ID 对应一个 ?,，3 个 ID 就重复 3 次，末尾会多一个逗号
	// strings.TrimRight是裁剪右侧逗号，比如：【"?,?,?,"】裁剪后变成：【"?,?,?"】
	placeholders := strings.TrimRight(strings.Repeat("?,", len(businessRoleIDs)), ",")
	querySQL := SQLQueriesPackage.SelectOpsRoleInternalIDsByRoleIDsPrefixSQL + "(" + placeholders + ")"
	args := make([]interface{}, 0, len(businessRoleIDs))
	for _, roleID := range businessRoleIDs {
		args = append(args, roleID)
	}

	rows, err := tx.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roleIDToBinding := make(map[string]opsRoleBinding, len(businessRoleIDs))
	for rows.Next() {
		var roleID string
		var internalID int64
		var name string
		var status int
		// role_id, internal_id 映射关系到roleID，internalID
		if err := rows.Scan(&roleID, &internalID, &name, &status); err != nil {
			return nil, err
		}
		roleIDToBinding[roleID] = opsRoleBinding{BusinessID: roleID, InternalID: internalID, Name: name, Status: status}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	bindings := make([]opsRoleBinding, 0, len(roleIDToBinding))
	for _, roleID := range businessRoleIDs {
		binding, ok := roleIDToBinding[roleID]
		if !ok || binding.InternalID <= 0 {
			return nil, fmt.Errorf("invalid roleID: %s", roleID)
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

/* 这段代码是 MySQL 特有的批量插入 + 重复主键忽略更新语法，常用于批量给管理员绑定角色，当 admin_user_id + role_id 联合唯一索引冲突时，不修改原有数据。 */
func insertAdminUserRolesBatch(ctx context.Context, tx *sql.Tx, adminUserID int64, bindings []opsRoleBinding) error {
	if len(bindings) == 0 {
		return nil
	}

	valueParts := make([]string, 0, len(bindings))
	args := make([]interface{}, 0, len(bindings)*2)
	for _, binding := range bindings {
		if binding.InternalID <= 0 {
			return fmt.Errorf("invalid roleID")
		}
		valueParts = append(valueParts, "(?, ?, NOW(), 0)")
		args = append(args, adminUserID, binding.InternalID)
	}

	/* 最终拼接完成后的sql语句是：
	INSERT INTO `admin_user_role` (`admin_user_id`, `role_id`, `update_at`, `update_by`) VALUES (?,?,?,?),(?,?,?,?),(?,?,?,?)

	这是批量插入写法，一次 SQL 插入多条记录，性能远高于循环单条 Insert
	所有 ? 占位符对应的真实参数全部存在切片 args 里，顺序一一对应
	*/
	querySQL := SQLQueriesPackage.InsertOpsAdminUserRoleBatchPrefixSQL + strings.Join(valueParts, ",")
	// 表 admin_user_role 一定建立了唯一联合索引， 同一个管理员不能重复绑定同一个角色
	// 当执行 INSERT 时，数据库检测到即将插入的数据违反唯一索引（重复记录），不会抛出主键冲突错误，转而执行 UPDATE 后面的逻辑
	querySQL += " ON DUPLICATE KEY UPDATE `role_id` = `role_id`"
	_, err := tx.ExecContext(ctx, querySQL, args...)
	return err
}

func insertUserRoleViewBatch(ctx context.Context, tx *sql.Tx, adminUserID int64, bindings []opsRoleBinding) error {
	if len(bindings) == 0 {
		return nil
	}

	valueParts := make([]string, 0, len(bindings))
	args := make([]interface{}, 0, len(bindings)*4)
	for _, binding := range bindings {
		if strings.TrimSpace(binding.BusinessID) == "" {
			return fmt.Errorf("invalid roleID")
		}
		valueParts = append(valueParts, "(?, ?, ?, ?, ?)")
		args = append(args, adminUserID, binding.BusinessID, binding.Name, binding.Status)
	}

	querySQL := SQLQueriesPackage.InsertOpsUserRoleViewBatchPrefixSQL + strings.Join(valueParts, ",")
	querySQL += " ON DUPLICATE KEY UPDATE `role_name` = VALUES(`role_name`), `status` = VALUES(`status`)"
	_, err := tx.ExecContext(ctx, querySQL, args...)
	return err
}

// GetUserRoles 获取用户角色
func (r *Repository) GetUserRoles(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	adminUserID, err := r.getAdminInternalIDByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectOpsUserRolesSQL, adminUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]map[string]interface{}, 0, 8)
	for rows.Next() {
		var roleID, name string
		var status int
		var createdAt time.Time
		if err := rows.Scan(&roleID, &name, &status, &createdAt); err != nil {
			return nil, err
		}
		list = append(list, map[string]interface{}{
			"id":          roleID,
			"name":        name,
			"description": "",
			"status":      status,
			"createdAt":   createdAt.Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Repository) getAdminInternalIDByUserID(ctx context.Context, userID string) (int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, fmt.Errorf("invalid userID")
	}

	var adminUserID int64
	if err := r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectOpsAdminInternalIDByUserIDSQL, userID).Scan(&adminUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("invalid userID")
		}
		return 0, err
	}
	if adminUserID <= 0 {
		return 0, fmt.Errorf("invalid userID")
	}
	return adminUserID, nil
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

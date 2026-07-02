/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 20:54:05
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-02 20:39:22
 * @FilePath: /MLC_GO/internal/pkg/mysql/queries/hg_sql_queries.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package SQLQueriesPackage

// 数据库
const (
	// 数据库dsn
	DB_DSN = `root:hh109@tcp(127.0.0.1:3306)/HG_MLC_DB?charset=utf8mb4&parseTime=True&loc=UTC`
)

// 用户模块
const (
	// InsertUserSQL 新增用户基础账号信息，写入业务 user_id、用户名、邮箱、手机号、密码哈希和盐。
	InsertUserSQL = `
	INSERT INTO users (user_id, user_name, email, phone, password_hash, salt) 
	VALUES (?, ?, ?, ?, ?, ?)`
	// GetUserByEmailOrPhoneSQL 登录时按邮箱或手机号查找用户认证信息。
	GetUserByEmailOrPhoneSQL = `SELECT user_id, user_name, email, phone, password_hash, salt 
	FROM users 
	WHERE email = ? OR phone = ?`
	// UserTotalNumSQL 统计 users 表总行数，供 offset 分页和 total 缓存回源使用。
	UserTotalNumSQL = `SELECT COUNT(*) FROM users`
	// InsertUserInfoSQL 仅插入邮箱、手机号、密码哈希和盐的历史 SQL，保留给旧调用兼容。
	InsertUserInfoSQL = `INSERT INTO users(email, phone, password_hash, salt) VALUES (?, ?, ?, ?)`
	// GetUserByIDSQL 按数据库自增主键 id 查询用户资料。
	GetUserByIDSQL = "SELECT id, user_id, user_name, nickname, signature, gender, birth_month, avatar_url, email, phone FROM users WHERE id = ?"
	// UpdateUserInfoByIDSQL 按数据库自增主键 id 更新用户邮箱和手机号。
	UpdateUserInfoByIDSQL = `UPDATE users SET email = ?, phone = ? WHERE id = ?`
	// UpdateUserInfoByUserIDSQL 按业务 user_id 更新用户邮箱和手机号，是对外用户接口优先使用的更新条件。
	UpdateUserInfoByUserIDSQL = `UPDATE users SET email = ?, phone = ? WHERE user_id = ?`
	// GetUserByUserIDSQL 按业务 user_id 查询用户资料。
	GetUserByUserIDSQL = "SELECT id, user_id, user_name, nickname, signature, gender, birth_month, avatar_url, email, phone FROM users WHERE user_id = ?"
	// SelectUserInfoByPhoneSQL 按手机号查询登录签发 token 所需的用户 id、业务 user_id 和密码字段。
	SelectUserInfoByPhoneSQL = `SELECT id, user_id, email, phone, password_hash, salt
	FROM users WHERE phone = ?`
	// QueryUserPageSQL 使用 LIMIT/OFFSET 做传统分页；数据量大或页码很深时会扫描较多行。
	QueryUserPageSQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users ORDER BY id DESC LIMIT ? OFFSET ?`
	// QueryUserPageFirstSQL 游标分页首屏查询，按 id 倒序取最新用户列表。
	QueryUserPageFirstSQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users ORDER BY id DESC LIMIT ?`
	// QueryUserPageV2SQL 游标分页下一页查询，取 id 小于 cursor 的数据，避免深分页 offset 扫描。
	QueryUserPageV2SQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users WHERE id < ? ORDER BY id DESC LIMIT ?`
	// GetUserByUsernameSQL 按用户名查询认证信息；注意当前 SQL 使用 username 字段名，需与表结构保持一致。
	GetUserByUsernameSQL = "SELECT user_id, username, email, phone, password_hash, salt FROM users WHERE username = ?"
	// UpdateUserPasswordSQL 按业务 user_id 更新密码哈希和盐。
	UpdateUserPasswordSQL = "UPDATE users SET password_hash = ?, salt = ? WHERE user_id = ?"
	// DeleteUserSQL 按业务 user_id 删除用户记录。
	DeleteUserSQL = "DELETE FROM users WHERE user_id = ?"
)

// 安全模块
const (
	// SelectUserSecurityBaseForUpdateSQL 在事务内锁定 users 行，并读取初始化 user_security 所需的认证字段。 FOR UPDATE 给查出来的行加锁（排他锁/X锁）
	SelectUserSecurityBaseForUpdateSQL = `SELECT user_id, email, phone, password_hash, salt FROM users WHERE user_id = ? FOR UPDATE`
	// SelectUserSecurityIDForUpdateSQL 在事务内锁定 user_security 行，用于判断安全记录是否已存在并防止并发写冲突。
	SelectUserSecurityIDForUpdateSQL = `SELECT id FROM user_security WHERE user_id = ? FOR UPDATE`
	// InsertUserSecuritySQL 创建用户安全记录，保存邮箱、手机号、密码哈希、盐、QQ 和微信等安全资料。
	InsertUserSecuritySQL = `INSERT INTO user_security (user_id, email, phone, password_hash, salt, qq, wechat) VALUES (?, ?, ?, ?, ?, ?, ?)`
	// SelectUserSecurityByUserIDSQL 按业务 user_id 读取账号安全表全部字段。
	SelectUserSecurityByUserIDSQL = `SELECT id, user_id, email, phone, password_hash, salt, qq, wechat, created_at, updated_at
	FROM user_security WHERE user_id = ?`
)

// 朋友圈模块
const ()

// 运维模块
const (
	// InsertOpsRoleSQL 创建角色。
	// 写入 role 表的业务 role_id 和基础字段；status 固定为 1，create_at/update_at 由数据库当前时间生成。
	// 索引/约束要求：role.name 需要唯一索引 uidx_name，避免并发创建同名角色。
	InsertOpsRoleSQL = "INSERT INTO `role` (`role_id`, `name`, `description`, `status`, `create_at`, `update_at`) VALUES (?, ?, ?, 1, NOW(), NOW())"

	// SelectOpsRoleListFirstSQL 获取角色首页列表。
	// 千万级表约束：使用 idx_status_id(status,id) 复合索引，固定 status=1 并按 id 倒序取 pageSize+1 条。
	// 不使用 COUNT(*)，不使用 OFFSET；多取 1 条仅用于判断 hasMore，避免深分页扫描和实时统计压力。
	SelectOpsRoleListFirstSQL = "SELECT `role_id`, `id`, `name`, `description`, `create_at` FROM `role` WHERE `status` = 1 ORDER BY `id` DESC LIMIT ?"

	// SelectOpsRoleListByCursorSQL 获取角色下一页列表。
	// cursor 是上一页最后一条 role.id；WHERE status=1 AND id < ? 可继续命中 idx_status_id(status,id)。
	// 该写法是大表 cursor 分页，避免 LIMIT/OFFSET 在千万级数据下扫描并丢弃大量行。
	SelectOpsRoleListByCursorSQL = "SELECT `role_id`, `id`, `name`, `description`, `create_at` FROM `role` WHERE `status` = 1 AND `id` < ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserWithoutEmailSQL 搜索管理员基础字段，不引用 email。
	// 对外返回的管理员 ID 必须使用 admin_user.user_id，它与 users.user_id 同类型、同语义；admin_user.id 只是后台表自增主键，不返回给角色分配搜索框。
	// 兼容旧库：部分环境 admin_user 尚未执行 email 加列迁移，使用该 SELECT 可避免 MySQL 1054 Unknown column。
	SelectOpsAdminUserWithoutEmailSQL = "SELECT `user_id`, `name`, `nick_name`, `mobile`, `status` FROM `admin_user`"

	// SelectOpsAdminUserWithEmailSQL 搜索管理员基础字段，包含 email。
	// 第一个字段固定为 user_id，保证 scanAdminUserRow 扫描出的 id 就是 users.user_id，而不是 admin_user 自增 id。
	// 仅在 hasAdminUserEmailColumn 检测到 admin_user.email 存在后使用；email 可命中 idx_email 前缀搜索。
	SelectOpsAdminUserWithEmailSQL = "SELECT `user_id`, `name`, `nick_name`, `email`, `mobile`, `status` FROM `admin_user`"

	// SelectOpsAdminUserListFirstWithoutEmailSQL 获取管理员首页列表，不引用 email 字段。
	// 千万级表约束：使用 is_delete 过滤和 id 倒序 cursor 分页，建议 admin_user 具备 (is_delete,id) 复合索引；不使用 COUNT/OFFSET。
	SelectOpsAdminUserListFirstWithoutEmailSQL = "SELECT `id`, `name`, `nick_name`, `mobile`, `status` FROM `admin_user` WHERE `is_delete` = 0 ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserListByCursorWithoutEmailSQL 获取管理员下一页列表，不引用 email 字段。
	// cursor 是上一页最后一条 admin_user.id；WHERE is_delete=0 AND id<? 避免深分页扫描和大量回表丢弃。
	SelectOpsAdminUserListByCursorWithoutEmailSQL = "SELECT `id`, `name`, `nick_name`, `mobile`, `status` FROM `admin_user` WHERE `is_delete` = 0 AND `id` < ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserListFirstWithEmailSQL 获取管理员首页列表，包含 email 字段。
	// 仅在 admin_user.email 字段存在时使用；分页策略与无 email 版本一致，不做实时总数统计。
	SelectOpsAdminUserListFirstWithEmailSQL = "SELECT `id`, `name`, `nick_name`, `email`, `mobile`, `status` FROM `admin_user` WHERE `is_delete` = 0 ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserListByCursorWithEmailSQL 获取管理员下一页列表，包含 email 字段。
	// cursor 采用自增主键 id，ORDER BY id DESC 保证分页稳定；建议建立 (is_delete,id) 索引支撑大表查询。
	SelectOpsAdminUserListByCursorWithEmailSQL = "SELECT `id`, `name`, `nick_name`, `email`, `mobile`, `status` FROM `admin_user` WHERE `is_delete` = 0 AND `id` < ? ORDER BY `id` DESC LIMIT ?"

	// OpsAdminUserActiveConditionSQL 管理员搜索的基础条件。
	// is_delete=0 必须始终存在，避免搜索结果包含软删除管理员。
	OpsAdminUserActiveConditionSQL = "`is_delete` = 0"

	// OpsAdminUserIDConditionSQL 管理员 ID 搜索条件。
	// 这里的“管理员 ID”指 admin_user.user_id，也就是 users.user_id；使用前缀 LIKE 支持输入完整或部分管理员 ID。
	OpsAdminUserIDConditionSQL = "`user_id` LIKE ?"

	// OpsAdminUserKeywordWithoutEmailConditionSQL 管理员关键词搜索条件，不包含 email。
	// 支持前端角色分配页输入姓名/昵称/手机号的全部或任意一部分；Repository 会传入 %keyword% 做模糊匹配，并用 limit 控制返回规模。
	OpsAdminUserKeywordWithoutEmailConditionSQL = "(`name` LIKE ? OR `nick_name` LIKE ? OR `mobile` LIKE ?)"

	// OpsAdminUserKeywordWithEmailConditionSQL 管理员关键词搜索条件，包含 email。
	// email 字段存在时启用；name/nick_name/email/mobile 均支持 %keyword% 模糊匹配，满足“可输入全部或一部分”的交互要求。
	OpsAdminUserKeywordWithEmailConditionSQL = "(`name` LIKE ? OR `nick_name` LIKE ? OR `email` LIKE ? OR `mobile` LIKE ?)"

	// SelectOpsAdminUserIDByUsersIDSQL 按 users.user_id 精确定位管理员 ID。
	// users.id 是数据库自增主键，不是对外用户 ID；这里必须返回 users.user_id，后续再用 admin_user.user_id 回表确认管理员存在且未删除。
	SelectOpsAdminUserIDByUsersIDSQL = "SELECT `user_id` FROM `users` WHERE `user_id` = ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserIDByUsersUserIDPrefixSQL 按 users.user_id 前缀定位管理员 ID。
	// 命中 users.user_id 唯一索引前缀；返回 users.user_id 后再按 admin_user.user_id 回表确认管理员存在且未删除。
	SelectOpsAdminUserIDByUsersUserIDPrefixSQL = "SELECT `user_id` FROM `users` WHERE `user_id` LIKE ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserIDByUsersUserNamePrefixSQL 按 users.user_name 前缀定位管理员 ID。
	// 搜索条件命中 user_name 索引，但返回值仍必须是 users.user_id，不能返回 users.id。
	SelectOpsAdminUserIDByUsersUserNamePrefixSQL = "SELECT `user_id` FROM `users` WHERE `user_name` LIKE ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserIDByUsersNickNamePrefixSQL 按 users.nickname 模糊定位管理员 ID。
	// 部分用户展示名只写在 nickname 字段；返回值仍必须是 users.user_id，避免回表时混用 users.id。
	SelectOpsAdminUserIDByUsersNickNamePrefixSQL = "SELECT `user_id` FROM `users` WHERE `nickname` LIKE ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserIDByUsersEmailPrefixSQL 按 users.email 前缀定位管理员 ID。
	// 命中 users.email/uk_email 唯一索引前缀；返回 users.user_id 供 admin_user.user_id 回表。
	SelectOpsAdminUserIDByUsersEmailPrefixSQL = "SELECT `user_id` FROM `users` WHERE `email` LIKE ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserIDByUsersPhonePrefixSQL 按 users.phone 前缀定位管理员 ID。
	// 命中 users.phone/uk_phone 唯一索引前缀；返回 users.user_id，避免把 users.id 误当作管理员业务 ID。
	SelectOpsAdminUserIDByUsersPhonePrefixSQL = "SELECT `user_id` FROM `users` WHERE `phone` LIKE ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserIDByCourseUserIDSQL 按课程平台 user.id 精确定位管理员 ID。
	// 命中 user 主键；用于 admin_user.id 与课程平台 user.id 共用同一 ID 的搜索链路。
	SelectOpsAdminUserIDByCourseUserIDSQL = "SELECT `id` FROM `user` WHERE `id` = ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserIDByCourseUserNickNamePrefixSQL 按课程平台 user.nick_name 前缀定位管理员 ID。
	// 命中 user.idx_name 前缀；返回 ID 后再按 admin_user 主键回表确认管理员存在且未删除。
	SelectOpsAdminUserIDByCourseUserNickNamePrefixSQL = "SELECT `id` FROM `user` WHERE `nick_name` LIKE ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminUserIDByWechatUserIDSQL 按 wechat_user.user_id 精确定位管理员 ID。
	// 命中 wechat_user.uidx_user 唯一索引；用于微信身份表与 user/admin_user 共用用户 ID 的搜索链路。
	SelectOpsAdminUserIDByWechatUserIDSQL = "SELECT `user_id` FROM `wechat_user` WHERE `user_id` = ? ORDER BY `user_id` DESC LIMIT ?"

	// SelectOpsAdminUserIDByAppUserIDSQL 按 app_user.user_id 精确定位管理员 ID。
	// 命中 app_user.uidx_user_appcode 唯一索引前缀；用于应用身份表与 user/admin_user 共用用户 ID 的搜索链路。
	SelectOpsAdminUserIDByAppUserIDSQL = "SELECT `user_id` FROM `app_user` WHERE `user_id` = ? ORDER BY `user_id` DESC LIMIT ?"

	// SelectOpsAdminCandidateByIDSQL 按 users.id 精确搜索添加管理员候选。
	// 命中 users 主键；用于纯数字关键词的最高选择性查询，避免 OR 条件导致优化器误判扫描范围。
	SelectOpsAdminCandidateByIDSQL = "SELECT `id`, `user_id`, `user_name`, `nickname`, `email`, `phone` FROM `users` WHERE `id` = ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminCandidateByUserIDPrefixSQL 按 users.user_id 前缀搜索候选。
	// 命中 users.user_id 唯一索引前缀；禁止 %keyword% 包含查询，避免千万级 users 表全表扫描。
	SelectOpsAdminCandidateByUserIDPrefixSQL = "SELECT `id`, `user_id`, `user_name`, `nickname`, `email`, `phone` FROM `users` WHERE `user_id` LIKE ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminCandidateByUserNamePrefixSQL 按 users.user_name 前缀搜索候选。
	// 命中 users.user_name 唯一索引前缀；单字段查询后由 Repository 合并去重，避免多字段 OR 放大扫描。
	SelectOpsAdminCandidateByUserNamePrefixSQL = "SELECT `id`, `user_id`, `user_name`, `nickname`, `email`, `phone` FROM `users` WHERE `user_name` LIKE ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminCandidateByEmailPrefixSQL 按 users.email 前缀搜索候选。
	// 命中 users.email/uk_email 唯一索引前缀；仅返回有限候选，不执行 COUNT/OFFSET。
	SelectOpsAdminCandidateByEmailPrefixSQL = "SELECT `id`, `user_id`, `user_name`, `nickname`, `email`, `phone` FROM `users` WHERE `email` LIKE ? ORDER BY `id` DESC LIMIT ?"

	// SelectOpsAdminCandidateByPhonePrefixSQL 按 users.phone 前缀搜索候选。
	// 命中 users.phone/uk_phone 唯一索引前缀；手机号搜索是添加管理员最常用路径之一。
	SelectOpsAdminCandidateByPhonePrefixSQL = "SELECT `id`, `user_id`, `user_name`, `nickname`, `email`, `phone` FROM `users` WHERE `phone` LIKE ? ORDER BY `id` DESC LIMIT ?"

	// InsertOpsAdminFromUserWithEmailSQL 从 users 主键提升为 admin_user，适用于 admin_user.email 已迁移的环境。
	// 写入 admin_user.user_id 时必须使用 users.user_id，保证新增管理员后 SearchAdminUsers 能按同一个业务用户 ID 搜索和返回。
	InsertOpsAdminFromUserWithEmailSQL = "INSERT INTO `admin_user` (`user_id`, `name`, `nick_name`, `email`, `mobile`, `lark_open_id`, `password`, `status`, `create_at`, `update_at`, `create_by`, `update_by`, `sex`, `is_delete`) SELECT `user_id`, COALESCE(NULLIF(`user_name`, ''), `user_id`, CONCAT('user_', `id`)), COALESCE(NULLIF(`nickname`, ''), NULLIF(`user_name`, ''), `user_id`, CONCAT('user_', `id`)), NULLIF(`email`, ''), COALESCE(NULLIF(`phone`, ''), CONCAT('user_', `id`)), '', `password_hash`, 1, NOW(), NOW(), ?, ?, 3, 0 FROM `users` WHERE `id` = ?"

	// InsertOpsAdminFromUserWithoutEmailSQL 从 users 主键提升为 admin_user，兼容 admin_user.email 尚未迁移的环境。
	// 不引用 admin_user.email，避免旧库报 MySQL 1054 Unknown column；仍写入 user_id，避免管理员搜索结果缺少业务用户 ID。
	InsertOpsAdminFromUserWithoutEmailSQL = "INSERT INTO `admin_user` (`user_id`, `name`, `nick_name`, `mobile`, `lark_open_id`, `password`, `status`, `create_at`, `update_at`, `create_by`, `update_by`, `sex`, `is_delete`) SELECT `user_id`, COALESCE(NULLIF(`user_name`, ''), `user_id`, CONCAT('user_', `id`)), COALESCE(NULLIF(`nickname`, ''), NULLIF(`user_name`, ''), `user_id`, CONCAT('user_', `id`)), COALESCE(NULLIF(`phone`, ''), CONCAT('user_', `id`)), '', `password_hash`, 1, NOW(), NOW(), ?, ?, 3, 0 FROM `users` WHERE `id` = ?"

	// SelectOpsAdminByMobileWithEmailSQL 按手机号读取管理员，包含 email 字段。
	// 命中 admin_user.idx_mobile 唯一索引，用于插入重复时返回已存在管理员信息。
	SelectOpsAdminByMobileWithEmailSQL = "SELECT `user_id`, `name`, `nick_name`, `email`, `mobile`, `status` FROM `admin_user` WHERE `mobile` = ? AND `is_delete` = 0 LIMIT 1"

	// SelectOpsAdminByMobileWithoutEmailSQL 按手机号读取管理员，不引用 email 字段。
	// 兼容旧库 admin_user.email 缺失场景，仍命中 admin_user.idx_mobile 唯一索引。
	SelectOpsAdminByMobileWithoutEmailSQL = "SELECT `user_id`, `name`, `nick_name`, `mobile`, `status` FROM `admin_user` WHERE `mobile` = ? AND `is_delete` = 0 LIMIT 1"

	// SelectOpsAdminByPrimaryIDWithEmailSQL 按 admin_user.id 自增主键读取管理员，包含 email 字段。
	// 仅用于 AddAdminFromUser 刚插入成功后通过 LastInsertId 回查；SELECT 仍返回 user_id，保证对外 DTO 的 id 是 users.user_id。
	SelectOpsAdminByPrimaryIDWithEmailSQL = "SELECT `user_id`, `name`, `nick_name`, `email`, `mobile`, `status` FROM `admin_user` WHERE `id` = ? AND `is_delete` = 0 LIMIT 1"

	// SelectOpsAdminByPrimaryIDWithoutEmailSQL 按 admin_user.id 自增主键读取管理员，不引用 email 字段。
	// 仅用于兼容 admin_user.email 尚未迁移环境下的新增管理员回查；对外仍返回 admin_user.user_id。
	SelectOpsAdminByPrimaryIDWithoutEmailSQL = "SELECT `user_id`, `name`, `nick_name`, `mobile`, `status` FROM `admin_user` WHERE `id` = ? AND `is_delete` = 0 LIMIT 1"

	// SelectOpsAdminByIDWithEmailSQL 按 admin_user.user_id 读取管理员，包含 email 字段。
	// user_id 与 users.user_id 类型一致，是角色分配搜索结果使用的对外管理员 ID；不要用 admin_user.id 自增主键。
	SelectOpsAdminByIDWithEmailSQL = "SELECT `user_id`, `name`, `nick_name`, `email`, `mobile`, `status` FROM `admin_user` WHERE `user_id` = ? AND `is_delete` = 0 LIMIT 1"

	// SelectOpsAdminByIDWithoutEmailSQL 按 admin_user.user_id 读取管理员，不引用 email 字段。
	// 兼容旧库 admin_user.email 缺失场景；查询条件仍使用 user_id，保证 SearchAdminUsers 的 id 字段始终对应 users.user_id。
	SelectOpsAdminByIDWithoutEmailSQL = "SELECT `user_id`, `name`, `nick_name`, `mobile`, `status` FROM `admin_user` WHERE `user_id` = ? AND `is_delete` = 0 LIMIT 1"

	// SelectOpsUserPhoneByIDSQL 按 users.id 读取手机号。
	// 命中 users 主键，用于 admin_user 唯一键冲突时回查已存在管理员。
	SelectOpsUserPhoneByIDSQL = "SELECT COALESCE(NULLIF(`phone`, ''), CONCAT('user_', `id`)) FROM `users` WHERE `id` = ?"

	// SelectOpsTableColumnExistsSQL 检查当前库表字段是否存在。
	// 查询 MySQL 元数据 INFORMATION_SCHEMA.COLUMNS，不扫描业务表；用于兼容灰度迁移期间字段可能缺失的环境。
	SelectOpsTableColumnExistsSQL = "SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?"

	// DeleteOpsAdminUserRolesSQL 删除指定管理员的旧角色集合。
	// 事务内执行；WHERE admin_user_id=? 命中 admin_user_role 唯一索引前缀，删除范围限定在单个管理员。
	DeleteOpsAdminUserRolesSQL = "DELETE FROM `admin_user_role` WHERE `admin_user_id` = ?"

	// DeleteOpsUserRoleViewSQL 删除指定管理员的角色冗余读模型。
	// AssignUserRoles 在同一事务内先删后写，保证 user_role_view 与 admin_user_role 保持一致。
	DeleteOpsUserRoleViewSQL = "DELETE FROM `user_role_view` WHERE `admin_user_id` = ?"

	// InsertOpsAdminUserRoleSQL 插入管理员角色关联。
	// 事务内批量执行；依赖唯一索引 (admin_user_id, role_id) 保证同一管理员不会重复绑定同一角色。
	InsertOpsAdminUserRoleSQL = "INSERT INTO `admin_user_role` (`admin_user_id`, `role_id`, `update_at`, `update_by`) VALUES (?, ?, NOW(), 0)"

	// SelectOpsRoleInternalIDsByRoleIDsPrefixSQL 按业务 role.role_id 批量映射内部 role.id。
	// role_id 是对外业务 ID；admin_user_role.role_id 仍保存内部自增 id，避免破坏现有关联表结构。
	SelectOpsRoleInternalIDsByRoleIDsPrefixSQL = "SELECT `role_id`, `id`, `name`, `status` FROM `role` WHERE `status` = 1 AND `role_id` IN "

	// InsertOpsAdminUserRoleBatchPrefixSQL 批量插入管理员角色关联。
	// Repository 动态拼接 VALUES 占位符，避免 N 次 SQL 执行和 N 次网络往返。
	InsertOpsAdminUserRoleBatchPrefixSQL = "INSERT INTO `admin_user_role` (`admin_user_id`, `role_id`, `update_at`, `update_by`) VALUES "

	// InsertOpsUserRoleViewBatchPrefixSQL 批量写入用户角色冗余读模型。
	// 写路径承担冗余成本，GetUserRoles 读路径直接按 admin_user_id 命中索引，避免大表 join。
	InsertOpsUserRoleViewBatchPrefixSQL = "INSERT INTO `user_role_view` (`admin_user_id`, `role_id`, `role_name`, `status`, `create_at`) VALUES "

	// SelectOpsUserRolesSQL 查询指定管理员已绑定角色。
	// 直接读取 user_role_view 冗余表，WHERE admin_user_id/status + ORDER BY id DESC 命中 idx_admin_status_id，避免亿级关联表 join。
	SelectOpsUserRolesSQL = "SELECT `role_id`, `role_name`, `status`, `create_at` FROM `user_role_view` WHERE `admin_user_id` = ? AND `status` = 1 ORDER BY `id` DESC"
)

// 视频模块
const (
	// InsertOrUpdateVideoSubmissionSQL 创建或更新稿件记录。
	// 第一个分 P 会创建 video_submissions；后续分 P 复用 submission_id 并累加数量和大小。
	InsertOrUpdateVideoSubmissionSQL = `
INSERT INTO video_submissions (
    submission_id, user_id, title, video_count, total_size, status
) VALUES (?, ?, ?, 1, ?, 'draft')
ON DUPLICATE KEY UPDATE
    video_count = video_count + 1,
    total_size = total_size + VALUES(total_size),
    updated_at = CURRENT_TIMESTAMP`

	// InsertOrUpdateVideoFileSQL 创建或更新视频文件记录。
	// 上传完成后写入文件信息，包括路径、大小、MD5 等。
	InsertOrUpdateVideoFileSQL = `
INSERT INTO video_files (
    video_id, submission_id, user_id, part_number, title, file_name, file_path,
    file_size, mime_type, md5, upload_status, upload_progress, transcode_status
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'completed', 100.00, 'pending')
ON DUPLICATE KEY UPDATE
    title = VALUES(title),
    file_name = VALUES(file_name),
    file_path = VALUES(file_path),
    file_size = VALUES(file_size),
    mime_type = VALUES(mime_type),
    md5 = VALUES(md5),
    upload_status = 'completed',
    upload_progress = 100.00,
    updated_at = CURRENT_TIMESTAMP`

	// SaveSubmissionSQL 保存完整稿件配置。
	// 因为本模块不使用外键，所有写入都带 userID/submissionID 限定，避免误更新其他用户数据。
	SaveSubmissionSQL = `
INSERT INTO video_submissions (
    submission_id, user_id, title, cover_url, category, video_type, source_url, description,
    allow_secondary_creation, watermark, visibility, declaration, card_config, dolby_audio,
    hires_audio, close_danmaku, close_comment, featured_comment, dynamic_description,
    hide_from_profile, video_count, total_size, status, submit_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    title = VALUES(title),
    cover_url = VALUES(cover_url),
    category = VALUES(category),
    video_type = VALUES(video_type),
    source_url = VALUES(source_url),
    description = VALUES(description),
    allow_secondary_creation = VALUES(allow_secondary_creation),
    watermark = VALUES(watermark),
    visibility = VALUES(visibility),
    declaration = VALUES(declaration),
    card_config = VALUES(card_config),
    dolby_audio = VALUES(dolby_audio),
    hires_audio = VALUES(hires_audio),
    close_danmaku = VALUES(close_danmaku),
    close_comment = VALUES(close_comment),
    featured_comment = VALUES(featured_comment),
    dynamic_description = VALUES(dynamic_description),
    hide_from_profile = VALUES(hide_from_profile),
    video_count = VALUES(video_count),
    total_size = VALUES(total_size),
    status = VALUES(status),
    submit_time = VALUES(submit_time),
    updated_at = CURRENT_TIMESTAMP`

	// GetSubmissionTotalsSQL 根据已上传的视频文件重新计算稿件总数和总大小。
	// 这样前端不需要可信地上报 video_count/total_size，避免被篡改。
	GetSubmissionTotalsSQL = `
SELECT COUNT(*), COALESCE(SUM(file_size), 0)
FROM video_files
WHERE submission_id = ? AND user_id = ?`

	// UpdateVideoFileConfigSQL 更新单个分 P 的表单配置。
	UpdateVideoFileConfigSQL = `
UPDATE video_files
SET part_number = ?, title = ?, cover_url = ?, video_type = ?, source_url = ?, category = ?, description = ?, updated_at = CURRENT_TIMESTAMP
WHERE video_id = ? AND submission_id = ? AND user_id = ?`

	// DeleteVideoTagsByVideoIDSQL 删除视频的所有标签。
	DeleteVideoTagsByVideoIDSQL = `DELETE FROM video_tags WHERE video_id = ?`

	// InsertVideoTagSQL 插入视频标签。
	InsertVideoTagSQL = `INSERT INTO video_tags (video_id, tag_name) VALUES (?, ?)`

	// DeleteScheduledPublishSQL 删除稿件的定时发布配置。
	DeleteScheduledPublishSQL = `DELETE FROM video_scheduled_publish WHERE submission_id = ? AND user_id = ?`

	// InsertOrUpdateScheduledPublishSQL 创建或更新定时发布配置。
	InsertOrUpdateScheduledPublishSQL = `
INSERT INTO video_scheduled_publish (submission_id, user_id, scheduled_time, status)
VALUES (?, ?, ?, 'pending')
ON DUPLICATE KEY UPDATE
    scheduled_time = VALUES(scheduled_time),
    status = 'pending',
    updated_at = CURRENT_TIMESTAMP`

	// DeleteCommercialPromotionSQL 删除稿件的商业推广配置。
	DeleteCommercialPromotionSQL = `DELETE FROM video_commercial_promotion WHERE submission_id = ? AND user_id = ?`

	// InsertOrUpdateCommercialPromotionSQL 创建或更新商业推广配置。
	InsertOrUpdateCommercialPromotionSQL = `
INSERT INTO video_commercial_promotion (submission_id, user_id, promotion_type, promotion_name, promotion_form)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    promotion_type = VALUES(promotion_type),
    promotion_name = VALUES(promotion_name),
    promotion_form = VALUES(promotion_form),
    updated_at = CURRENT_TIMESTAMP`

	// GetVideoListSQL 获取已提交审核的视频列表。
	// 使用延迟关联优化深分页，先查主键再回表，避免千万级数据量下 OFFSET 扫描过多行。
	// 索引要求：video_submissions 表需要 (status, submit_time) 联合索引。
	GetVideoListSQL = `
SELECT
    vs.submission_id,
    vs.user_id,
    vs.title,
    vs.cover_url,
    vs.category,
    vs.video_type,
    vs.description,
    vs.visibility,
    vs.status,
    vs.video_count,
    vs.total_size,
    vs.submit_time,
    vs.created_at,
    vf.video_id,
    vf.file_path,
    vf.file_name,
    vf.file_size,
    vf.mime_type,
    vf.part_number
FROM video_submissions vs
INNER JOIN (
    SELECT submission_id
    FROM video_submissions
    WHERE status IN ('reviewing', 'published')
    ORDER BY submit_time DESC
    LIMIT ? OFFSET ?
) AS vs_page ON vs.submission_id = vs_page.submission_id
LEFT JOIN video_files vf ON vs.submission_id = vf.submission_id AND vf.part_number = 1
ORDER BY vs.submit_time DESC`

	// GetVideoListTotalSQL 获取已提交审核的视频总数。
	// 使用近似值优化：当数据量超过一定阈值时，可改用 SHOW TABLE STATUS 或缓存。
	GetVideoListTotalSQL = `
SELECT COUNT(*)
FROM video_submissions
WHERE status IN ('reviewing', 'published')`

	// CreateVideoSubmissionStatusTimeIndexSQL 创建视频稿件状态和提交时间联合索引。
	// 用于优化 GetVideoListSQL 的查询性能。
	CreateVideoSubmissionStatusTimeIndexSQL = `
CREATE INDEX IF NOT EXISTS idx_video_submissions_status_submit_time
ON video_submissions (status, submit_time DESC)`

	// GetVideoListByCursorFirstSQL 游标分页首页查询。
	// 使用 (status, submit_time DESC) 联合索引覆盖排序，避免 filesort。
	// 多查一条用于判断是否还有下一页。
	GetVideoListByCursorFirstSQL = `
SELECT
    vs.submission_id,
    vs.user_id,
    vs.title,
    vs.cover_url,
    vs.category,
    vs.video_type,
    vs.description,
    vs.visibility,
    vs.status,
    vs.video_count,
    vs.total_size,
    vs.submit_time,
    vs.created_at,
    vf.video_id,
    vf.file_path,
    vf.file_name,
    vf.file_size,
    vf.mime_type,
    vf.part_number
FROM video_submissions vs
INNER JOIN (
    SELECT submission_id
    FROM video_submissions
    WHERE status IN ('reviewing', 'published')
    ORDER BY submit_time DESC, submission_id DESC
    LIMIT ?
) AS vs_page ON vs.submission_id = vs_page.submission_id
LEFT JOIN video_files vf ON vs.submission_id = vf.submission_id AND vf.part_number = 1
ORDER BY vs.submit_time DESC, vs.submission_id DESC`

	// GetVideoListByCursorSQL 游标分页翻页查询。
	// 使用 (submit_time, submission_id) 复合游标定位，避免 OFFSET 扫描。
	// submit_time 相同时用 submission_id 保证分页结果稳定不丢不重。
	GetVideoListByCursorSQL = `
SELECT
    vs.submission_id,
    vs.user_id,
    vs.title,
    vs.cover_url,
    vs.category,
    vs.video_type,
    vs.description,
    vs.visibility,
    vs.status,
    vs.video_count,
    vs.total_size,
    vs.submit_time,
    vs.created_at,
    vf.video_id,
    vf.file_path,
    vf.file_name,
    vf.file_size,
    vf.mime_type,
    vf.part_number
FROM video_submissions vs
INNER JOIN (
    SELECT submission_id
    FROM video_submissions
    WHERE status IN ('reviewing', 'published')
      AND (submit_time < ? OR (submit_time = ? AND submission_id < ?))
    ORDER BY submit_time DESC, submission_id DESC
    LIMIT ?
) AS vs_page ON vs.submission_id = vs_page.submission_id
LEFT JOIN video_files vf ON vs.submission_id = vf.submission_id AND vf.part_number = 1
ORDER BY vs.submit_time DESC, vs.submission_id DESC`
)

// 聊天模块
const ()

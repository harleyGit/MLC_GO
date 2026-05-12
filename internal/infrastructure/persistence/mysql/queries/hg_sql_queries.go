/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 20:54:05
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-11 10:11:37
 * @FilePath: /MLC_GO/internal/infrastructure/persistence/mysql/queries/hg_sql_queries.go
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
)

// 朋友圈模块
const ()

// 视频模块
const ()

// 聊天模块
const ()

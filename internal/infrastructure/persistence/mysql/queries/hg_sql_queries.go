/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 20:54:05
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-03-01 22:21:23
 * @FilePath: /MLC_GO/internal/infrastructure/persistence/mysql/queries/hg_sql_queries.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package SQLQueriesPackage

const (

	// 数据库dsn
	DB_DSN = `root:hh109@tcp(127.0.0.1:3306)/HG_MLC_DB?charset=utf8mb4&parseTime=True&loc=UTC`
	// 用户相关SQL语句
	InsertUserSQL = `
	INSERT INTO users (user_id, user_name, email, phone, password_hash, salt) 
	VALUES (?, ?, ?, ?, ?, ?)`

	// 查询用户信息
	GetUserByEmailOrPhoneSQL = `SELECT user_id, user_name, email, phone, password_hash, salt 
	FROM users 
	WHERE email = ? OR phone = ?`
	// 用户总数
	UserTotalNumSQL = `SELECT COUNT(*) FROM users`

	InsertUserInfoSQL        = `INSERT INTO users(email, phone, password_hash, salt) VALUES (?, ?, ?, ?)`
	UpdateUserInfoSQL        = `UPDATE users SET email = ?, phone = ?, WHERE user_id = ?`
	GetUserByIDSQL           = "SELECT user_id, user_name, nickname, signature, gender, birth_month, avatar_url, email, phone FROM users WHERE user_id = ?"
	SelectUserInfoByPhoneSQL = `SELECT id, email, phone, password_hash, salt
	FROM users WHERE phone = ?`
	// 用户分页查询【在十万级别数据还可以，百万以上不行】
	QueryUserPageSQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users ORDER BY id DESC LIMIT ? OFFSET ?`
	// 用户列表首屏查询，按主键倒序取最新数据。
	QueryUserPageFirstSQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users ORDER BY id DESC LIMIT ?`
	// 用户分页查询，使用游标查询优化，避免深分页 offset 扫描。
	QueryUserPageV2SQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users WHERE id < ? ORDER BY id DESC LIMIT ?`

	GetUserByUsernameSQL  = "SELECT user_id, username, email, phone, password_hash, salt FROM users WHERE username = ?"
	UpdateUserPasswordSQL = "UPDATE users SET password_hash = ?, salt = ? WHERE user_id = ?"
	DeleteUserSQL         = "DELETE FROM users WHERE user_id = ?"
)

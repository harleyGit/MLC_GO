/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 20:54:05
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-10 21:13:13
 * @FilePath: /MLC_GO/internal/infrastructure/persistence/mysql/queries/hg_sql_queries.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package SQLQueriesPackage

const (
	// 数据库
	DB_DSN = `root:hh109@tcp(127.0.0.1:3306)/HG_MLC_DB?charset=utf8mb4&parseTime=True&loc=UTC`

	// 用户模块
	InsertUserSQL = `
	INSERT INTO users (user_id, user_name, email, phone, password_hash, salt) 
	VALUES (?, ?, ?, ?, ?, ?)`
	GetUserByEmailOrPhoneSQL = `SELECT user_id, user_name, email, phone, password_hash, salt 
	FROM users 
	WHERE email = ? OR phone = ?`
	UserTotalNumSQL = `SELECT COUNT(*) FROM users`
	InsertUserInfoSQL = `INSERT INTO users(email, phone, password_hash, salt) VALUES (?, ?, ?, ?)`
	UpdateUserInfoByIDSQL = `UPDATE users SET email = ?, phone = ? WHERE id = ?`
	UpdateUserInfoByUserIDSQL = `UPDATE users SET email = ?, phone = ? WHERE user_id = ?`
	GetUserByIDSQL = "SELECT id, user_id, user_name, nickname, signature, gender, birth_month, avatar_url, email, phone FROM users WHERE id = ?"
	GetUserByUserIDSQL = "SELECT id, user_id, user_name, nickname, signature, gender, birth_month, avatar_url, email, phone FROM users WHERE user_id = ?"
	SelectUserInfoByPhoneSQL = `SELECT id, user_id, email, phone, password_hash, salt
	FROM users WHERE phone = ?`
	QueryUserPageSQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users ORDER BY id DESC LIMIT ? OFFSET ?`
	QueryUserPageFirstSQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users ORDER BY id DESC LIMIT ?`
	QueryUserPageV2SQL = `SELECT id, user_id, user_name, email, phone, password_hash, salt, created_at, updated_at
	FROM users WHERE id < ? ORDER BY id DESC LIMIT ?`
	GetUserByUsernameSQL = "SELECT user_id, username, email, phone, password_hash, salt FROM users WHERE username = ?"
	UpdateUserPasswordSQL = "UPDATE users SET password_hash = ?, salt = ? WHERE user_id = ?"
	DeleteUserSQL = "DELETE FROM users WHERE user_id = ?"

	// 安全模块
	SelectUserSecurityBaseForUpdateSQL = `SELECT user_id, email, phone, password_hash, salt FROM users WHERE user_id = ? FOR UPDATE`
	SelectUserSecurityIDForUpdateSQL = `SELECT id FROM user_security WHERE user_id = ? FOR UPDATE`
	InsertUserSecuritySQL = `INSERT INTO user_security (user_id, email, phone, password_hash, salt, qq, wechat) VALUES (?, ?, ?, ?, ?, ?, ?)`

	// 朋友圈模块

	// 视频模块

	// 聊天模块
)

/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 20:22:42
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-15 10:55:52
 * @FilePath: /MLC_GO/internal/infrastructure/persistence/mysql/sql.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package PersistenceSQLPackage

import (
	SQLQueriesPackage "MLC_GO/internal/infrastructure/persistence/mysql/queries"
	UserModelsPackage "MLC_GO/internal/models/user_models"
	"MLC_GO/internal/pkg/logHG"
	"database/sql"
)

var (
	db *sql.DB)

func NewSQLDB() {
	var err error
	dsn := SQLQueriesPackage.DB_DSN
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		logHG.FatalFInfo("连接MySQL数据库失败: %v", err)
	}
	if err = db.Ping(); err != nil {
		logHG.FatalFInfo("Ping MySQL数据库失败: %v", err)
	}
}

/* 创建用户 */
func CreateUser(u *UserModelsPackage.HGUserModel) error {
	stmt, err := db.Exec(SQLQueriesPackage.InsertUserSQL, 
		u.Email, u.Phone, u.PasswordHash, u.Salt)
	if err != nil {
		return err
	}
	_, err = stmt.LastInsertId()
	return err
}
/* 获取用户信息 */
func GetUserByEmail(account string) (*UserModelsPackage.HGUserModel, error) {
	row := db.QueryRow(SQLQueriesPackage.GetUserByEmailOrPhoneSQL, account, account)

	u := &UserModelsPackage.HGUserModel{}
	err := row.Scan(&u.UserID, &u.Email, &u.Phone, &u.PasswordHash, &u.Salt)
	if err != nil {
		return nil, err
	}
	return u, nil
}
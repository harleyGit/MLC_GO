/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-13 10:52:50
  - @LastEditors: GangHuang harleysor@qq.com
  - @LastEditTime: 2026-01-21 16:26:14

* @FilePath: /MLC_GO/internal/models/user_models/hg_user_model.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* 数据库Model(只管DB)
*/
package UserModelsPackage

import "database/sql"

type HGUserModel struct {
	ID           int64
	UserID       sql.NullString
	Username     sql.NullString
	Email        sql.NullString //TODO: 完全不需要这个类型，可以直接用一个model，然后类型用指针就好了
	Phone        sql.NullString
	Password     sql.NullString
	PasswordHash sql.NullString
	Salt         sql.NullString
}

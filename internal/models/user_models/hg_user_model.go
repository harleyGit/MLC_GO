/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-13 10:52:50
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 20:13:43
 * @FilePath: /MLC_GO/internal/models/user_models/hg_user_model.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package usermodelsPackage
type HGUserModel struct {
	UserID   int64
	Username string
	Email    string
	Phone	string
	PasswordHash string
	Salt   string
}
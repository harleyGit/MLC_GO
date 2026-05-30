/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 20:04:43
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-31 22:33:20
 * @FilePath: /MLC_GO/internal/modules/user/dto/hg_auth.dto.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserDtoPackage

type SendCodeDTO struct {
	Phone string `json:"phone"`
}

type LoginDTO struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type LoginResultDTO struct {
	UserID       int64  `json:"user_id"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

/* 注册响应结构体 */
type RegisterReqModel struct {
	Account  string `json:"username"` // 前端传的是 username，映射到 Account
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

// ResetPasswordReqModel 忘记密码请求结构体。
// Phone 和 Code 用于身份验证，NewPassword 为用户新设置的密码。
type ResetPasswordReqModel struct {
	Phone       string `json:"phone"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

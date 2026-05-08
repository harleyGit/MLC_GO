/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-21 16:00:24
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-08 21:01:51

* @FilePath: /MLC_GO/internal/modules/user/dto/hg_user_dto.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* DTO(只管接口/业务)
*/

package UserDtoPackage

import HGResponsePakcage "MLC_GO/internal/response"

// 创建用户（POST）
// TODO: 定一个基类结构体，有 Code和message，然dto有这2个变量，方便在协议返回。否则这样写死了
type HGCreateUserDTO struct { // 让 JSON 响应“只包含有意义的数据”，提升 API 的简洁性、安全性、可维护性和用户体验。
	ID           int64   `json:"id,omitempty"` //omitempty 当字段是“空值”时，JSON 序列化时不输出该字段。
	UserID       *string `json:"user_id,omitempty"`
	Username     *string `json:"user_name,omitempty"`
	Nickname     *string `json:"nickname,omitempty"`
	Signature    *string `json:"signature,omitempty"`
	Gender       *int    `json:"gender,omitempty"`
	BirthMonth   *string `json:"birth_month,omitempty"`
	AvatarURL    *string `json:"avatar_url,omitempty"`
	Email        *string `json:"email,omitempty"`
	Phone        *string `json:"phone,omitempty"`
	Code         *string `json:"code,omitempty"`
	Password     string  `json:"password,omitempty"`
	PasswordHash *string `json:"passwordHash,omitempty"`
	Salt         *string `json:"salt,omitempty"`
	Token        *string `json:"token,omitempty"`
	Created_at   *string `json:"created_at,omitempty"`
	Updated_at   *string `json:"updated_at,omitempty"`
}

/* 实现协议可选 */
func (HGCreateUserDTO) ResponseCode() HGResponsePakcage.HGErrorCode {
	return HGResponsePakcage.OKCode
}

func (HGCreateUserDTO) ResponseMessage() string {
	return "success💯"
}

// HGUpdateUserProfileReqDTO 定义用户资料更新请求，字段均可选，支持单字段或多字段更新。
type HGUpdateUserProfileReqDTO struct {
	Nickname  *string `json:"nickname,omitempty"`
	Signature *string `json:"signature,omitempty"`
	Gender    *int    `json:"gender,omitempty"`
	BirthDate *string `json:"birth_date,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

// HasAnyField 判断更新请求是否至少包含一个可更新字段。
func (d *HGUpdateUserProfileReqDTO) HasAnyField() bool {
	if d == nil {
		return false
	}

	return d.Nickname != nil ||
		d.Signature != nil ||
		d.Gender != nil ||
		d.BirthDate != nil ||
		d.AvatarURL != nil
}

// HGUpdateUserProfileRespDTO 定义用户资料更新成功后的返回结构。
type HGUpdateUserProfileRespDTO struct {
	UserID    string  `json:"user_id"`
	Nickname  *string `json:"nickname,omitempty"`
	Signature *string `json:"signature,omitempty"`
	Gender    *int    `json:"gender,omitempty"`
	BirthDate *string `json:"birth_date,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

// HGUpdateUserSecurityReqDTO 定义账号安全信息更新请求，字段均可选，支持一次修改多个字段。
type HGUpdateUserSecurityReqDTO struct {
	QQ       *string `json:"qq,omitempty"`
	Password *string `json:"password,omitempty"`
	Phone    *string `json:"phone,omitempty"`
	Email    *string `json:"email,omitempty"`
	Wechat   *string `json:"wechat,omitempty"`
}

// HasAnyField 判断安全信息更新请求是否至少包含一个可更新字段。
func (d *HGUpdateUserSecurityReqDTO) HasAnyField() bool {
	if d == nil {
		return false
	}

	return d.QQ != nil ||
		d.Password != nil ||
		d.Phone != nil ||
		d.Email != nil ||
		d.Wechat != nil
}

// HGUpdateUserSecurityRespDTO 定义账号安全信息更新成功后的返回结构，不返回密码哈希和盐。
type HGUpdateUserSecurityRespDTO struct {
	UserID          string  `json:"user_id"`
	QQ              *string `json:"qq,omitempty"`
	Phone           *string `json:"phone,omitempty"`
	Email           *string `json:"email,omitempty"`
	Wechat          *string `json:"wechat,omitempty"`
	PasswordUpdated bool    `json:"password_updated"`
}

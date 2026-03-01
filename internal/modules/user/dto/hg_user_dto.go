/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-21 16:00:24
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-31 21:47:57

* @FilePath: /MLC_GO/internal/modules/user/dto/hg_user_dto.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* DTO(只管接口/业务)
*/

package UserDtoPackage

import HGResponsePakcage "MLC_GO/internal/response"

// 创建用户（POST）
// TODO: 定一个基类结构体，有 Code和message，然dto有这2个变量，方便在协议返回。否则这样写死了
type HGCreateUserDTO struct { // 让 JSON 响应“只包含有意义的数据”，提升 API 的简洁性、安全性、可维护性和用户体验。
	ID           int64   `json:"id,omitempty"`
	UserID       *string `json:"userID,omitempty"`
	Username     *string `json:"userName,omitempty"`
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

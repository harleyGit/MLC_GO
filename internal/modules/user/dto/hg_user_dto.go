/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-21 16:00:24
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-28 20:52:50

* @FilePath: /MLC_GO/internal/modules/user/dto/hg_user_dto.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* DTO(只管接口/业务)
*/

package UserDtoPackage

import HGResponsePakcage "MLC_GO/internal/response"

// 创建用户（POST）
// TODO: 定一个基类结构体，有 Code和message，然dto有这2个变量，方便在协议返回。否则这样写死了
type HGCreateUserDTO struct {
	ID           int64
	UserID       *string `json:"userID"`
	Username     *string `json:"userName"`
	Email        *string `json:"emial"`
	Phone        *string `json:"phone"`
	Code         *string `json:"code"`
	Passowrd     string  `json:"password"`
	PasswordHash *string `json:"passwordHash"`
	Salt         *string `json:"salt"`
	Token        *string `json:"token"`
}

/* 实现协议可选 */
func (HGCreateUserDTO) ResponseCode() HGResponsePakcage.HGErrorCode {
	return HGResponsePakcage.OKCode
}

func (HGCreateUserDTO) ResponseMessage() string {
	return "success💯"
}

/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-21 16:00:24
  - @LastEditors: GangHuang harleysor@qq.com
  - @LastEditTime: 2026-01-21 16:54:14

* @FilePath: /MLC_GO/internal/modules/user/dto/hg_user_dto.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* DTO(只管接口/业务)
*/
package UserDtoPackage

// 创建用户（POST）
type HGCreateUserDTO struct {
	ID           int64
	UserID       *string `json:"userID"`
	Username     *string `json:"userName"`
	Email        *string `json:"emial"`
	Phone        *string `json:"phone"`
	Passowrd     string `json:"password"`
	PasswordHash *string  `json:"passwordHash"`
	Salt         *string `json:"salt"`
}

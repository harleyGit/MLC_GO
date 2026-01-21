/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 20:04:43
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 20:05:48
 * @FilePath: /MLC_GO/internal/modules/user/dto/hg_auth.dto.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserDtoPackage

type SendCodeDTO struct {
	Phone string `json:"phone"`
}

type LoginDTO struct {
	Phone string `json:"phone"`
	Code string `json:"code"`
}

type LoginResultDTO struct {
	UserID int64 `json:"user_id"`
	Token string `json:"token"`
}
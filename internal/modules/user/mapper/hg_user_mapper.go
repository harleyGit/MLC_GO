/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 16:06:20
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-03-01 22:31:58
 * @FilePath: /MLC_GO/internal/modules/user/mapper/hg_user_mapper.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserMapperPackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"database/sql"
)

func UserDTOToModel(d *UserDtoPackage.HGCreateUserDTO) *UserModelsPackage.HGUserModel {
	return &UserModelsPackage.HGUserModel{
		UserID:       UtilsPackage.StrPtrToNullStr(d.UserID),
		Username:     UtilsPackage.StrPtrToNullStr(d.Username),
		Email:        UtilsPackage.StrPtrToNullStr(d.Email),
		Phone:        UtilsPackage.StrPtrToNullStr(d.Phone),
		PasswordHash: UtilsPackage.StrPtrToNullStr(d.PasswordHash),
		Salt:         UtilsPackage.StrPtrToNullStr(d.Salt),
	}
}

func PatchUserDTOToModel(d *UserDtoPackage.HGCreateUserDTO, u *UserModelsPackage.HGUserModel) {
	u.Email = UtilsPackage.StrPtrToNullStr(*&d.Email)
	u.Phone = UtilsPackage.StrPtrToNullStr(*&d.Phone)
}

func UserModelToDTO(u *UserModelsPackage.HGUserModel) *UserDtoPackage.HGCreateUserDTO {
	var birthMonth *string
	if u.BirthMonth.Valid {
		s := u.BirthMonth.Time.Format("2006-01-02")
		birthMonth = &s
	}

	return &UserDtoPackage.HGCreateUserDTO{
		ID:           u.ID,
		UserID:       UtilsPackage.NullStrToPtr(u.UserID),
		Username:     UtilsPackage.NullStrToPtr(u.Username),
		Nickname:     UtilsPackage.NullStrToPtr(u.Nickname),
		Signature:    UtilsPackage.NullStrToPtr(u.Signature),
		Gender:       nullInt64ToPtr(u.Gender),
		BirthMonth:   birthMonth,
		AvatarURL:    UtilsPackage.NullStrToPtr(u.AvatarURL),
		Email:        UtilsPackage.NullStrToPtr(u.Email),
		Phone:        UtilsPackage.NullStrToPtr(u.Phone),
		Created_at:   UtilsPackage.NullStrToPtr(u.CreatedAt),
		Updated_at:   UtilsPackage.NullStrToPtr(u.UpdatedAt),
	}
}

func nullInt64ToPtr(v sql.NullInt64) *int {
	if v.Valid {
		i := int(v.Int64)
		return &i
	}
	return nil
}

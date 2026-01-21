/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 20:23:05
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 20:47:58
 * @FilePath: /MLC_GO/internal/modules/user/service/hg_auth_service.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserServicePackage

import (
	UserCachePackage "MLC_GO/internal/modules/user/cache"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	UserRepositoryPackage "MLC_GO/internal/modules/user/repository"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"errors"
)

type HGAuthService struct {
	users *UserRepositoryPackage.UserRepo
	codes *UserCachePackage.HGCodeCache
}

func NewAuthService(
	users *UserRepositoryPackage.UserRepo,
	codes *UserCachePackage.HGCodeCache,
) *HGAuthService {
	return &HGAuthService{
		users: users,
		codes: codes,
	}
}

func (s *HGAuthService) SendCode(ctx context.Context, d *UserDtoPackage.SendCodeDTO) error {
	code := "123456" //TODO: 实际用随机值
	return s.codes.SetCode(ctx, d.Phone, code)
}
func (s *HGAuthService) Login(ctx context.Context, d *UserDtoPackage.LoginDTO) (*UserDtoPackage.LoginResultDTO, error) {
	code, err := s.codes.GetCode(ctx, d.Phone)
	if err != nil || code != d.Code {
		return nil, errors.New("验证码错误")
	}
	user, err := s.users.GetByPhone(ctx, d.Phone)
	if err != nil {
		// 用户不存在 -> 注册
		phone := d.Phone
		user = &UserModelsPackage.HGUserModel{
			Phone: UtilsPackage.StrPtrToNullStr(&phone),
		}
		if err := s.users.Insert(ctx, user); err != nil {
			return nil, err
		}
	}

	s.codes.DeleteCode(ctx, d.Phone)

	return &UserDtoPackage.LoginResultDTO{
		UserID: user.ID,
		Token:  "虚拟token......", //TODO：问问ai如何生辰token
	}, nil
}

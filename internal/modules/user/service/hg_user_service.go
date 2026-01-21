/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-13 10:54:52
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 20:22:39
 * @FilePath: /MLC_GO/internal/modules/user/service/hg_user_service.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 
 * 功能：业务逻辑层
 */
package UserServicePackage

import (
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserMapperPackage "MLC_GO/internal/modules/user/mapper"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	UserRepositoryPackage "MLC_GO/internal/modules/user/repository"
	utilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"strings"
)

type UserService struct {
	repo *UserRepositoryPackage.UserRepo
}

func NewUserService(repo *UserRepositoryPackage.UserRepo) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) CreateUser(ctx context.Context, d *UserDtoPackage.HGCreateUserDTO) error {
	salt := utilsPackage.GenerateRandomNum(8)
	hash := utilsPackage.HashPassword(d.Passowrd, salt)
	d.Salt = &salt
	d.PasswordHash = &hash

	user := UserMapperPackage.UserDTOToModel(d)

	return s.repo.Insert(ctx, user)
}

func (s *UserService) PathUser(
	ctx context.Context,
	id int64,
	d *UserDtoPackage.HGCreateUserDTO,
) (*UserDtoPackage.HGCreateUserDTO, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	UserMapperPackage.PatchUserDTOToModel(d, user)

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return UserMapperPackage.UserModelToDTO(user), nil
}

func RegisterService(account, code, password string) error {
	key := "verify:" + account
	v, err := PersistenceRedisPackage.GetFromRedis(key)
	if err != nil || v != code {
		return err
	}

	salt := utilsPackage.GenerateRandomNum(8)
	hashStr := utilsPackage.HashPassword(password, salt)
	u := &UserModelsPackage.HGUserModel{
		PasswordHash: utilsPackage.StrPtrToNullStr(&hashStr),
		Salt:         utilsPackage.StrPtrToNullStr(&salt),
	}

	if strings.Contains(account, "@") {
		u.Email = utilsPackage.StrPtrToNullStr(&account)
		// u.Email.Valid = true
	} else {
		u.Phone = utilsPackage.StrPtrToNullStr(&account)
		// u.Phone.Valid = true
	}

	err = PersistenceSQLPackage.CreateUser(u)
	if err == nil {
		PersistenceRedisPackage.DeleteFromRedis(key)
	}
	return err
}

func LoginService(account, password string) (string, error) {
	u, err := PersistenceSQLPackage.GetUserByEmail(account)
	if err != nil {
		return "", err
	}

	hashedPassword := utilsPackage.HashPassword(password, u.Salt.String)
	if hashedPassword != u.PasswordHash.String {
		return "", err
	}
	token := utilsPackage.GenerateRandomNum(16)
	err = PersistenceRedisPackage.SetToRedis("token:"+token, u.UserID, 24*3600)

	return token, nil
}

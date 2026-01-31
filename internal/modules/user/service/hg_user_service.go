/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-13 10:54:52
  - @LastEditors: GangHuang harleysor@qq.com
  - @LastEditTime: 2026-01-31 23:41:39

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
	UtilsPackage "MLC_GO/internal/pkg/utils"
	utilsPackage "MLC_GO/internal/pkg/utils"
	"context"
)

type UserService struct {
	repo *UserRepositoryPackage.UserRepo
}

func NewUserService(repo *UserRepositoryPackage.UserRepo) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) CreateUser(ctx context.Context, d *UserDtoPackage.HGCreateUserDTO) error {
	salt := utilsPackage.GenerateRandomNum(8)
	hash := utilsPackage.HashPassword(d.Password, salt)
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

func RegisterService(ctx context.Context, reigisterModel UserDtoPackage.RegisterReqModel) error {
	key := PersistenceRedisPackage.GetRedisVerifyCodeKey(reigisterModel.Phone)
	v, err := PersistenceRedisPackage.GetFromRedis(ctx, key)
	if err != nil || v != reigisterModel.Code {
		return err
	}
	// TODO: 密码判空处理
	salt := utilsPackage.GenerateRandomNum(8)
	hashStr := utilsPackage.HashPassword(reigisterModel.Password, salt)
	userID := UtilsPackage.GenerateUserID()
	userName := reigisterModel.Account
	if UtilsPackage.IsEmpty(userName) {
		userName = UtilsPackage.GenerateMultilingualName()
	}
	u := &UserModelsPackage.HGUserModel{
		UserID:       utilsPackage.StrPtrToNullStr(&userID),
		Username:     utilsPackage.StrPtrToNullStr(&userName),
		PasswordHash: utilsPackage.StrPtrToNullStr(&hashStr),
		Salt:         utilsPackage.StrPtrToNullStr(&salt),
		Phone:        UtilsPackage.StrPtrToNullStr(&reigisterModel.Phone),
		Email:        UtilsPackage.StrPtrToNullStr(&reigisterModel.Email),
	}

	err = PersistenceSQLPackage.CreateUser(u)
	if err == nil {
		// 删除注册时发送的验证码
		PersistenceRedisPackage.DeleteFromRedis(key)
	}
	return err
}

func LoginService(account, password string) (string, error) {
	u, err := PersistenceSQLPackage.GetUserByEmail(nil, account)
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

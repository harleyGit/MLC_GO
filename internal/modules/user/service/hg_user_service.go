/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-13 10:54:52
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-03-01 22:28:01

* @FilePath: /MLC_GO/internal/modules/user/service/hg_user_service.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* 功能：业务逻辑层
*/
package UserServicePackage

import (
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	HGUserCachePackage "MLC_GO/internal/modules/user/cache"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserMapperPackage "MLC_GO/internal/modules/user/mapper"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	UserRepositoryPackage "MLC_GO/internal/modules/user/repository"
	"MLC_GO/internal/pkg/logHG"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	utilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"context"
	// "encoding/json"
)

type UserService struct {
	repo      *UserRepositoryPackage.UserRepo //sql逻辑处理
	userCache *HGUserCachePackage.HGUserCache //缓存处理
}

func NewUserService(repo *UserRepositoryPackage.UserRepo,
	userCache *HGUserCachePackage.HGUserCache) *UserService {

	return &UserService{repo: repo, userCache: userCache}
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
		// Email:        UtilsPackage.StrPtrToNullStr(&reigisterModel.Email),
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

/* 获取注册的用户列表 */
func (s *UserService) GetUserList(ctx context.Context, page, size int) (HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO], error) {

	// ===== 1. Redis =====
	// if !UtilsPackage.IsEmpty(s.userCache) {
	// 	cacheValue, err := s.userCache.GetUserListCache(ctx, page, size)
	// 	if err != nil {
	// 		logHG.DebugFInfo("GetUserListCache err: %v", err)
	// 		return HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]{}, err
	// 	}

	// 	if !UtilsPackage.IsEmpty(cacheValue) {
	// 		var userList HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]
	// 		if err := json.Unmarshal([]byte(cacheValue), &userList); err != nil {
	// 			return HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]{}, err
	// 		}
	// 		return userList, nil
	// 	}
	// }

	users, total, err := s.repo.FindPage(ctx, page, size)
	if err != nil {
		return HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]{}, err
	}
	var dtoList []*UserDtoPackage.HGCreateUserDTO
	for _, user := range users {
		dtoList = append(dtoList, UserMapperPackage.UserModelToDTO(&user))
	}

	resp := HGResponsePakcage.NewPageResponse[*UserDtoPackage.HGCreateUserDTO](dtoList,
		HGResponsePakcage.WithPage(page),
		HGResponsePakcage.WithTotal(total),
		HGResponsePakcage.WithNumPages(page))

	if s.userCache != nil {
		err := s.userCache.SetUserListCache(ctx, resp, page, size)
		if err != nil {
			logHG.DebugFInfo("SetUserListCache err: %v", err)
		}
	}

	return resp, nil
}

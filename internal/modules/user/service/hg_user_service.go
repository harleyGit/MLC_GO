/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-13 10:54:52
  - @LastEditors: GangHuang harleysor@qq.com
  - @LastEditTime: 2026-04-15 20:44:43

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
	"encoding/json"
	"errors"
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

// RegisterService 负责注册验证码校验、落库与注册后缓存清理。
func RegisterService(ctx context.Context, reigisterModel UserDtoPackage.RegisterReqModel) error {
	key := PersistenceRedisPackage.GetRedisVerifyCodeKey(reigisterModel.Phone)
	v, err := PersistenceRedisPackage.GetFromRedis(ctx, key)
	if err != nil {
		return err
	}
	redisCode := decodeRedisStringValue(v)
	if redisCode != reigisterModel.Code {
		return errors.New("验证码错误 or 已过期")
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
		if delErr := PersistenceRedisPackage.DeleteFromRedis(
			key,
			PersistenceRedisPackage.WithContext(ctx),
		); delErr != nil {
			logHG.DebugFInfo("Delete register verify code cache err: %v", delErr)
		}

		// 注册成功后，删除用户列表缓存，避免 GetUserList 命中旧数据。
		if delErr := PersistenceRedisPackage.DeleteFromRedis(
			PersistenceRedisPackage.UserListTotalKey,
			PersistenceRedisPackage.WithContext(ctx),
		); delErr != nil {
			logHG.DebugFInfo("Delete user list total cache err: %v", delErr)
		}
		if delErr := PersistenceRedisPackage.DeleteFromRedisByPattern(
			PersistenceRedisPackage.UserListPatternKey,
			PersistenceRedisPackage.WithContext(ctx),
		); delErr != nil {
			logHG.DebugFInfo("Delete user list page cache err: %v", delErr)
		}
	}
	return err
}

// decodeRedisStringValue 兼容 Redis 中字符串值被 JSON 序列化后带引号的场景。
func decodeRedisStringValue(v string) string {
	var result string
	if err := json.Unmarshal([]byte(v), &result); err == nil {
		return result
	}
	return v
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
func (s *UserService) GetUserList(
	ctx context.Context,
	cursor int64,
	size int,
) (HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO], error) {

	// 先查 Redis。
	// cursor 分页的缓存 key 使用 cursor + size，避免大 offset 导致缓存命中率和 MySQL 性能都变差。
	if !UtilsPackage.IsEmpty(s.userCache) {
		cacheValue, err := s.userCache.GetUserListCache(ctx, cursor, size)
		if err != nil {
			logHG.DebugFInfo("GetUserListCache err: %v", err)
			return HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]{}, err
		}

		if cacheValue != nil {
			return *cacheValue, nil
		}
	}

	users, nextCursor, hasMore, err := s.repo.FindByCursor(ctx, cursor, size)
	if err != nil {
		return HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]{}, err
	}

	total, err := s.getUserListTotal(ctx)
	if err != nil {
		return HGResponsePakcage.HGPageResultModel[*UserDtoPackage.HGCreateUserDTO]{}, err
	}

	var dtoList []*UserDtoPackage.HGCreateUserDTO
	for _, user := range users {
		dtoList = append(dtoList, UserMapperPackage.UserModelToDTO(&user))
	}

	// 兼容现有响应结构中的 page 字段：
	// cursor 首页返回 1，其余页因为已经是游标模型，不再强行模拟 offset 页码。
	page := 1
	if cursor > 0 {
		page = 0
	}

	resp := HGResponsePakcage.NewPageResponse[*UserDtoPackage.HGCreateUserDTO](dtoList,
		HGResponsePakcage.WithPagesize(size),
		HGResponsePakcage.WithPage(page),
		HGResponsePakcage.WithTotal(total),
		HGResponsePakcage.WithNextCursor(nextCursor),
		HGResponsePakcage.WithHasMore(hasMore))

	if s.userCache != nil {
		err := s.userCache.SetUserListCache(ctx, resp, cursor, size)
		if err != nil {
			logHG.DebugFInfo("SetUserListCache err: %v", err)
		}
	}

	return resp, nil
}

func (s *UserService) getUserListTotal(ctx context.Context) (int, error) {
	if UtilsPackage.IsEmpty(s.userCache) {
		return s.repo.CountUsers(ctx)
	}

	total, err := s.userCache.GetUserListTotalCache(ctx)
	if err != nil {
		logHG.DebugFInfo("GetUserListTotalCache err: %v", err)
		return 0, err
	}
	if total > 0 {
		return total, nil
	}

	total, err = s.repo.CountUsers(ctx)
	if err != nil {
		return 0, err
	}

	if err = s.userCache.SetUserListTotalCache(ctx, total); err != nil {
		logHG.DebugFInfo("SetUserListTotalCache err: %v", err)
	}

	return total, nil
}

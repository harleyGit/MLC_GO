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
	"strings"
	"time"
)

type UserService struct {
	repo      *UserRepositoryPackage.UserRepo //sql逻辑处理
	userCache *HGUserCachePackage.HGUserCache //缓存处理
}

var (
	// ErrProfileNoField 表示更新资料请求未包含任何可更新字段。
	ErrProfileNoField = errors.New("至少更新一个资料字段")
	// ErrProfileGenderInvalid 表示性别字段超出允许范围。
	ErrProfileGenderInvalid = errors.New("gender 仅支持 0/1/2")
	// ErrProfileBirthDateInvalid 表示出生日期格式不符合约定。
	ErrProfileBirthDateInvalid = errors.New("birth_date 仅支持 YYYY-MM-DD 或 YYYY-MM")
)

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

// GetUserByID 根据用户ID获取用户信息（直接使用 string 类型的 user_id 查询）
func (s *UserService) GetUserByID(ctx context.Context, userID string) (*UserDtoPackage.HGCreateUserDTO, error) {
	if userID == "" {
		return nil, errors.New("user_id 不能为空")
	}

	user, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return UserMapperPackage.UserModelToDTO(user), nil
}

// GetByEmailOrPhone 根据邮箱或手机号获取用户信息
func (s *UserService) GetByEmailOrPhone(ctx context.Context, account string) (*UserModelsPackage.HGUserModel, error) {
	if account == "" {
		return nil, errors.New("account 不能为空")
	}

	return s.repo.GetByEmailOrPhone(ctx, account)
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

// UpdateProfile 更新用户资料，支持单字段或多字段更新。
func (s *UserService) UpdateProfile(
	ctx context.Context,
	userID string,
	d *UserDtoPackage.HGUpdateUserProfileReqDTO,
) (*UserDtoPackage.HGUpdateUserProfileRespDTO, error) {
	if d == nil || !d.HasAnyField() {
		return nil, ErrProfileNoField
	}

	if d.Gender != nil && (*d.Gender < 0 || *d.Gender > 2) {
		return nil, ErrProfileGenderInvalid
	}

	if d.BirthDate != nil {
		normalizedDate, err := normalizeBirthDate(*d.BirthDate)
		if err != nil {
			return nil, err
		}
		d.BirthDate = &normalizedDate
	}

	if err := s.repo.UpdateProfileByID(ctx, userID, d); err != nil {
		return nil, err
	}

	s.clearUserListCache(ctx)

	return &UserDtoPackage.HGUpdateUserProfileRespDTO{
		UserID:    userID,
		Nickname:  d.Nickname,
		Signature: d.Signature,
		Gender:    d.Gender,
		BirthDate: d.BirthDate,
		AvatarURL: d.AvatarURL,
	}, nil
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

// normalizeBirthDate 统一规范出生日期格式，支持 YYYY-MM-DD 与 YYYY-MM 输入。
func normalizeBirthDate(raw string) (string, error) {
	birthDate := strings.TrimSpace(raw)
	if birthDate == "" {
		return "", ErrProfileBirthDateInvalid
	}

	if parsedDate, err := time.Parse("2006-01-02", birthDate); err == nil {
		return parsedDate.Format("2006-01-02"), nil
	}

	if parsedMonth, err := time.Parse("2006-01", birthDate); err == nil {
		return parsedMonth.Format("2006-01") + "-01", nil
	}

	return "", ErrProfileBirthDateInvalid
}

// clearUserListCache 在用户资料写操作后清理列表分页缓存和总数缓存。
func (s *UserService) clearUserListCache(ctx context.Context) {
	if PersistenceRedisPackage.RDB == nil {
		return
	}

	if err := PersistenceRedisPackage.DeleteFromRedis(
		PersistenceRedisPackage.UserListTotalKey,
		PersistenceRedisPackage.WithContext(ctx),
	); err != nil {
		logHG.DebugFInfo("Delete user list total cache err: %v", err)
	}

	if err := PersistenceRedisPackage.DeleteFromRedisByPattern(
		PersistenceRedisPackage.UserListPatternKey,
		PersistenceRedisPackage.WithContext(ctx),
	); err != nil {
		logHG.DebugFInfo("Delete user list page cache err: %v", err)
	}
}

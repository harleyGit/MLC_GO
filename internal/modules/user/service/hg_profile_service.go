package UserServicePackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserMapperPackage "MLC_GO/internal/modules/user/mapper"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	utilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"errors"
	"strings"
	"time"
)

// CreateUser 创建用户并写入数据库，负责密码加盐哈希和 DTO 到 model 的转换。
// 该方法是基础用户创建能力；公开注册流程应使用 Register，以保证验证码和缓存清理完整执行。
func (s *UserService) CreateUser(ctx context.Context, d *UserDtoPackage.HGCreateUserDTO) error {
	salt := utilsPackage.GenerateRandomNum(8)
	hash := utilsPackage.HashPassword(d.Password, salt)
	d.Salt = &salt
	d.PasswordHash = &hash

	user := UserMapperPackage.UserDTOToModel(d)
	return s.repo.Insert(ctx, user)
}

// GetUserByID 根据用户 ID 获取用户资料。
// user_id 在当前表设计中是字符串业务 ID，不使用自增主键暴露给外部。
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

// GetByEmailOrPhone 根据邮箱或手机号获取用户模型。
// 该能力供认证流程复用，避免登录逻辑直接访问 repository。
func (s *UserService) GetByEmailOrPhone(ctx context.Context, account string) (*UserModelsPackage.HGUserModel, error) {
	if account == "" {
		return nil, errors.New("account 不能为空")
	}

	return s.repo.GetByEmailOrPhone(ctx, account)
}

// PathUser 按自增 ID 局部更新用户基础信息。
// 保留该方法用于兼容历史调用；新资料更新优先使用 UpdateProfile 按 user_id 更新。
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
// 写操作成功后必须清理用户列表分页缓存和 total 缓存，避免列表接口读到旧数据。
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

// normalizeBirthDate 统一规范出生日期格式，支持 YYYY-MM-DD 与 YYYY-MM 输入。
// 月份输入统一补齐为当月 1 日，保证数据库中日期格式稳定。
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

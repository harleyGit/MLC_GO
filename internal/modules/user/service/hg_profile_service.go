package UserServicePackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserMapperPackage "MLC_GO/internal/modules/user/mapper"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	hg_time "MLC_GO/internal/pkg/hg_time"
	utilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"errors"
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

// PathUser 按业务 user_id 局部更新用户基础信息。
// 对外接口统一使用 user_id，避免前端误传数据库自增主键 id。
func (s *UserService) PathUser(
	ctx context.Context,
	userID string,
	d *UserDtoPackage.HGCreateUserDTO,
) (*UserDtoPackage.HGCreateUserDTO, error) {
	if userID == "" {
		return nil, errors.New("user_id 不能为空")
	}

	user, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	UserMapperPackage.PatchUserDTOToModel(d, user)

	if err := s.repo.UpdateByUserID(ctx, userID, user); err != nil {
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
		normalizedDate, err := hg_time.NormalizeBirthDate(*d.BirthDate)
		if err != nil {
			return nil, err
		}
		d.BirthDate = &hg_time.ClientTime{Value: normalizedDate, Format: "date"}
	}

	if err := s.repo.UpdateProfileByUserID(ctx, userID, d); err != nil {
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

package UserServicePackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	userrepository "MLC_GO/internal/modules/user/repository"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"database/sql"
	"errors"
	"strings"
)

var (
	// ErrSecurityNoField 表示账号安全请求未包含任何可更新字段。
	ErrSecurityNoField = errors.New("至少更新一个账号安全字段")
	// ErrSecurityFieldEmpty 表示请求中显式传入的安全字段为空。
	ErrSecurityFieldEmpty = errors.New("账号安全字段不能为空")
	// ErrSecurityDuplicate 表示邮箱、手机号、QQ 或微信号已被占用。
	ErrSecurityDuplicate = errors.New("账号安全字段已被占用")
)

// UpdateSecurity 更新当前用户账号安全信息，支持 QQ、密码、手机、邮箱、微信号单字段或多字段更新。
// 密码在 service 层完成加盐哈希，handler 和 repository 不接触明文密码返回值。
func (s *UserService) UpdateSecurity(
	ctx context.Context,
	userID string,
	d *UserDtoPackage.HGUpdateUserSecurityReqDTO,
) (*UserDtoPackage.HGUpdateUserSecurityRespDTO, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user_id 不能为空")
	}
	if d == nil || !d.HasAnyField() {
		return nil, ErrSecurityNoField
	}
	if err := normalizeSecurityRequest(d); err != nil {
		return nil, err
	}

	var passwordHash *string
	var salt *string
	if d.Password != nil {
		generatedSalt := UtilsPackage.GenerateRandomNum(8)
		hashedPassword := UtilsPackage.HashPassword(*d.Password, generatedSalt)
		passwordHash = &hashedPassword
		salt = &generatedSalt
	}

	if err := s.repo.UpdateSecurityByUserID(ctx, userID, d, passwordHash, salt); err != nil {
		if errors.Is(err, userrepository.ErrUserSecurityDuplicate) {
			return nil, ErrSecurityDuplicate
		}
		return nil, err
	}
	s.clearUserListCache(ctx)

	return &UserDtoPackage.HGUpdateUserSecurityRespDTO{
		UserID:          userID,
		QQ:              d.QQ,
		Phone:           d.Phone,
		Email:           d.Email,
		Wechat:          d.Wechat,
		PasswordUpdated: d.Password != nil,
	}, nil
}

// GetSecurityInfo 获取当前用户账号安全表完整字段。
func (s *UserService) GetSecurityInfo(ctx context.Context, userID string) (*UserDtoPackage.HGUserSecurityInfoRespDTO, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user_id 不能为空")
	}

	security, err := s.repo.GetSecurityByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return userSecurityModelToDTO(security), nil
}

func userSecurityModelToDTO(security *UserModelsPackage.HGUserSecurityModel) *UserDtoPackage.HGUserSecurityInfoRespDTO {
	if security == nil {
		return nil
	}

	return &UserDtoPackage.HGUserSecurityInfoRespDTO{
		ID:           security.ID,
		UserID:       security.UserID,
		Email:        nullStringPtr(security.Email),
		Phone:        nullStringPtr(security.Phone),
		PasswordHash: nullStringPtr(security.PasswordHash),
		Salt:         nullStringPtr(security.Salt),
		QQ:           nullStringPtr(security.QQ),
		Wechat:       nullStringPtr(security.Wechat),
		CreatedAt:    nullStringPtr(security.CreatedAt),
		UpdatedAt:    nullStringPtr(security.UpdatedAt),
	}
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}

	return &v.String
}

// normalizeSecurityRequest 统一清理账号安全字段，避免空白字符串写入唯一索引字段。
func normalizeSecurityRequest(d *UserDtoPackage.HGUpdateUserSecurityReqDTO) error {
	if d.QQ != nil {
		if err := trimRequiredString(d.QQ); err != nil {
			return err
		}
	}
	if d.Password != nil {
		if strings.TrimSpace(*d.Password) == "" {
			return ErrSecurityFieldEmpty
		}
	}
	if d.Phone != nil {
		if err := trimRequiredString(d.Phone); err != nil {
			return err
		}
	}
	if d.Email != nil {
		if err := trimRequiredString(d.Email); err != nil {
			return err
		}
	}
	if d.Wechat != nil {
		if err := trimRequiredString(d.Wechat); err != nil {
			return err
		}
	}

	return nil
}

// trimRequiredString 对可选但一旦传入就不能为空的字符串字段做原地规范化。
func trimRequiredString(v *string) error {
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return ErrSecurityFieldEmpty
	}
	*v = trimmed
	return nil
}

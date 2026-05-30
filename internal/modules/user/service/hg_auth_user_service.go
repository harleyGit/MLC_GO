package UserServicePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	"MLC_GO/internal/pkg/logHG"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	utilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Register 负责注册验证码校验、用户落库与注册后缓存清理。
// service 只编排业务流程，不直接写 HTTP 响应；数据库写入统一通过注入的 repository 完成。
func (s *UserService) Register(ctx context.Context, registerModel UserDtoPackage.RegisterReqModel) error {
	if s == nil || s.repo == nil || s.redisService == nil {
		return errors.New("user service dependency is nil")
	}
	if registerModel.Phone == "" || registerModel.Code == "" || registerModel.Password == "" {
		return errors.New("手机号、验证码和密码不能为空")
	}

	key := PersistenceRedisPackage.GetRedisVerifyCodeKey(registerModel.Phone)
	v, err := s.redisService.GetFromRedisV2(key, ctx)
	if err != nil {
		return err
	}
	if decodeRedisStringValue(v) != registerModel.Code {
		return errors.New("验证码错误 or 已过期")
	}

	salt := utilsPackage.GenerateRandomNum(8)
	hashStr := utilsPackage.HashPassword(registerModel.Password, salt)
	userID := UtilsPackage.GenerateUserID()
	userName := registerModel.Account
	if UtilsPackage.IsEmpty(userName) {
		userName = UtilsPackage.GenerateMultilingualName()
	}

	u := &UserModelsPackage.HGUserModel{
		UserID:       utilsPackage.StrPtrToNullStr(&userID),
		Username:     utilsPackage.StrPtrToNullStr(&userName),
		PasswordHash: utilsPackage.StrPtrToNullStr(&hashStr),
		Salt:         utilsPackage.StrPtrToNullStr(&salt),
		Phone:        UtilsPackage.StrPtrToNullStr(&registerModel.Phone),
	}
	if registerModel.Email != "" {
		u.Email = UtilsPackage.StrPtrToNullStr(&registerModel.Email)
	}

	if err = s.repo.Insert(ctx, u); err != nil {
		return err
	}
	if delErr := s.redisService.DeleteFromRedis(key, ctx); delErr != nil {
		logHG.DebugFInfo("Delete register verify code cache err: %v", delErr)
	}
	s.clearUserListCache(ctx)
	return nil
}

// LoginRequest 是 service 层登录入参，由 handler 从 HTTP 请求体转换而来。
// Phone/Email 二选一；Code 和 Password 二选一，优先验证码登录。
type LoginRequest struct {
	Phone    *string `json:"phone,omitempty"`
	Email    *string `json:"email,omitempty"`
	Code     *string `json:"code,omitempty"`
	Password *string `json:"password,omitempty"`
	Device   string  `json:"device,omitempty"`
}

// LoginResponse 是 service 层登录出参，handler 直接作为统一响应 data 返回。
type LoginResponse struct {
	UserID       string `json:"user_id"`
	UserName     string `json:"user_name,omitempty"`
	Nickname     string `json:"nickname,omitempty"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// Login 用户登录，支持验证码和密码两种方式。
// 认证成功后签发 access/refresh token，并把 token 状态写入 Redis 以支持撤销和刷新。
func (s *UserService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	if s == nil || s.repo == nil || s.redisService == nil || s.redisService.Client() == nil {
		return nil, errors.New("user service dependency is nil")
	}
	if req == nil {
		return nil, errors.New("登录请求不能为空")
	}
	if UtilsPackage.IsEmpty(req.Phone) && UtilsPackage.IsEmpty(req.Email) {
		return nil, ErrPhoneOrEmailRequired
	}

	var account string
	if !UtilsPackage.IsEmpty(req.Phone) {
		account = *req.Phone
	} else {
		account = *req.Email
	}

	userModel, err := s.repo.GetByEmailOrPhone(ctx, account)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if UtilsPackage.IsEmpty(req.Code) {
		if UtilsPackage.IsEmpty(req.Password) {
			return nil, ErrPasswordIncorrect
		}
		hashedPassword := utilsPackage.HashPassword(*req.Password, userModel.Salt.String)
		if hashedPassword != userModel.PasswordHash.String {
			return nil, ErrPasswordIncorrect
		}
	} else {
		cacheKey := PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.AuthLoginVerifyCodekKey, account)
		val, err := s.redisService.GetFromRedisV2(cacheKey, ctx)
		if err != nil {
			return nil, ErrCodeInvalid
		}
		if decodeRedisStringValue(val) != *req.Code {
			return nil, ErrCodeInvalid
		}
		// 验证码为一次性凭证，验证成功后立即删除，降低重放风险。
		s.redisService.DeleteFromRedis(cacheKey, ctx)
	}

	tokenPair, err := GenerateTokens(ctx, s.redisService.Client(), userModel.UserID.String, req.Device, "user")
	if err != nil {
		return nil, fmt.Errorf("生成 token 失败: %w", err)
	}

	return &LoginResponse{
		UserID:       userModel.UserID.String,
		UserName:     userModel.Username.String,
		Nickname:     userModel.Nickname.String,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}

// SendCode 生成并缓存验证码。
// Redis 中字符串值可能被 JSON 序列化，读取比较时必须使用 decodeRedisStringValue 做兼容。
func (s *UserService) SendCode(ctx context.Context, phone string) (string, error) {
	if phone == "" {
		return "", errors.New("手机号不能为空")
	}

	code := utilsPackage.GenerateRandomNum(6)
	key := PersistenceRedisPackage.GetRedisVerifyCodeKey(phone)
	if err := s.redisService.SetToRedisV2(key, code, time.Minute, ctx); err != nil {
		return "", errors.New("redis error")
	}

	logHG.DebugFInfo("验证码发送到 phone %s:，验证码： %s， 1分钟过期", phone, code)
	return code, nil
}

// SendResetPasswordCode 发送忘记密码验证码。
// 使用独立的 Redis key（AuthResetPasswordCodeKey），与注册/登录验证码隔离，避免互相覆盖。
func (s *UserService) SendResetPasswordCode(ctx context.Context, phone string) (string, error) {
	if phone == "" {
		return "", errors.New("手机号不能为空")
	}

	// 检查用户是否存在
	if _, err := s.repo.GetByEmailOrPhone(ctx, phone); err != nil {
		return "", ErrUserNotFound
	}

	code := utilsPackage.GenerateRandomNum(6)
	key := PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.AuthResetPasswordCodeKey, phone)
	if err := s.redisService.SetToRedisV2(key, code, time.Minute*5, ctx); err != nil {
		return "", errors.New("redis error")
	}

	logHG.DebugFInfo("忘记密码验证码发送到 phone %s:，验证码： %s， 5分钟过期", phone, code)
	return code, nil
}

// ResetPassword 忘记密码：通过手机验证码验证身份后重置密码。
// 流程：校验参数 → 查找用户 → 校验验证码 → 生成新密码哈希 → 事务更新密码 → 清理缓存。
// 高并发设计：验证码一次性消费、密码更新走事务保证原子性、复用现有 UpdateSecurityByUserID 避免重复代码。
func (s *UserService) ResetPassword(ctx context.Context, req *UserDtoPackage.ResetPasswordReqModel) error {
	if s == nil || s.repo == nil || s.redisService == nil {
		return errors.New("user service dependency is nil")
	}
	if req.Phone == "" {
		return errors.New("手机号不能为空")
	}
	if req.Code == "" {
		return errors.New("验证码不能为空")
	}
	if req.NewPassword == "" {
		return errors.New("新密码不能为空")
	}
	if len(req.NewPassword) < 6 {
		return errors.New("密码长度不能少于6位")
	}

	// 查找用户是否存在
	userModel, err := s.repo.GetByEmailOrPhone(ctx, req.Phone)
	if err != nil {
		return ErrUserNotFound
	}

	// 校验忘记密码验证码
	cacheKey := PersistenceRedisPackage.GetCacheKey(PersistenceRedisPackage.AuthResetPasswordCodeKey, req.Phone)
	val, err := s.redisService.GetFromRedisV2(cacheKey, ctx)
	if err != nil {
		return ErrCodeInvalid
	}
	if decodeRedisStringValue(val) != req.Code {
		return ErrCodeInvalid
	}

	// 验证码一次性消费，验证成功后立即删除
	s.redisService.DeleteFromRedis(cacheKey, ctx)

	// 生成新密码哈希
	salt := utilsPackage.GenerateRandomNum(8)
	hashedPassword := utilsPackage.HashPassword(req.NewPassword, salt)

	// 复用 UpdateSecurityByUserID 事务更新密码，保证 users + user_security 两表一致性
	securityDTO := &UserDtoPackage.HGUpdateUserSecurityReqDTO{
		Password: &req.NewPassword,
	}
	if err := s.repo.UpdateSecurityByUserID(ctx, userModel.UserID.String, securityDTO, &hashedPassword, &salt); err != nil {
		return fmt.Errorf("重置密码失败: %w", err)
	}

	s.clearUserListCache(ctx)
	return nil
}

// decodeRedisStringValue 兼容 Redis 中字符串值被 JSON 序列化后带引号的场景。
// 例如 SetToRedisV2("123456") 实际可能保存为 JSON 字符串 `"123456"`，比较验证码前必须解码。
func decodeRedisStringValue(v string) string {
	var result string
	if err := json.Unmarshal([]byte(v), &result); err == nil {
		return result
	}
	return v
}

// RegisterService 兼容旧函数名。
// Deprecated: 新代码必须使用 UserService.Register，避免绕过依赖注入和 repository。
func RegisterService(ctx context.Context, registerModel UserDtoPackage.RegisterReqModel) error {
	return errors.New("RegisterService 已废弃，请使用 UserService.Register")
}

// LoginService 兼容旧函数名。
// Deprecated: 新代码必须使用 UserService.Login，避免绕过依赖注入和 repository。
func LoginService(account, password string) (string, error) {
	return "", errors.New("LoginService 已废弃，请使用 UserService.Login")
}

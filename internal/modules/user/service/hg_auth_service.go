/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 20:23:05
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-25 19:25:40
 * @FilePath: /MLC_GO/internal/modules/user/service/hg_auth_service.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserServicePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	HGSMSPackage "MLC_GO/internal/modules/sms"
	HGSMSCachePackage "MLC_GO/internal/modules/sms/cache"
	UserCachePackage "MLC_GO/internal/modules/user/cache"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	UserRepositoryPackage "MLC_GO/internal/modules/user/repository"
	"MLC_GO/internal/pkg/logHG"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type HGAuthService struct {
	users   *UserRepositoryPackage.UserRepo
	codes   *UserCachePackage.HGCodeCache
	limiter *HGSMSCachePackage.HGSMSRateLimiter
	sms     *HGSMSPackage.HGPhoneSMSSender
	rdb     *redis.Client
}

// NewAuthService 创建认证服务。
func NewAuthService(
	users *UserRepositoryPackage.UserRepo,
	codes *UserCachePackage.HGCodeCache,
) *HGAuthService {
	return &HGAuthService{
		users: users,
		codes: codes,
	}
}

// Token 结构定义
var (
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 7 * 24 * time.Hour
	Secret     = []byte("change-me") // jwt 签名密钥，生产环境应从配置中心或环境变量读取
)

// HGClaims JWT Claims 结构。
type HGClaims struct {
	UserID  string `json:"uid"`            // 用户 ID
	Device  string `json:"device"`         // 设备信息（支持多端登录控制）
	JTI     string `json:"jti"`            // JWT 唯一 ID（防重放攻击）
	TokenTp string `json:"tp"`             // Token 类型：access / refresh
	Role    string `json:"role,omitempty"` // 用户角色
	jwt.RegisteredClaims
}

// TokenPair Token 对。
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// randJTI 生成随机的 JWT ID（32 字节十六进制字符串）。
func randJTI() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateTokens 生成 Access Token 和 Refresh Token（大厂标准实现）。
// 参数：
//   - ctx: 上下文
//   - rdb: Redis 客户端
//   - userID: 用户 ID
//   - device: 设备信息（可选，用于多端登录控制）
//   - role: 用户角色（可选，默认 "user"）
func GenerateTokens(
	ctx context.Context,
	rdb *redis.Client,
	userID string,
	device string,
	role string,
) (*TokenPair, error) {
	if userID == "" {
		return nil, errors.New("userID 不能为空")
	}

	if role == "" {
		role = "user"
	}

	now := time.Now()
	jti := randJTI()

	// 生成 Access Token
	accessClaims := &HGClaims{
		UserID:  userID,
		Device:  device,
		JTI:     jti,
		TokenTp: "access",
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "mlc-go",
			Subject:   "user-token",
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(Secret)
	if err != nil {
		return nil, fmt.Errorf("生成 access token 失败: %w", err)
	}

	// 生成 Refresh Token（使用独立的 JTI）
	refreshJTI := randJTI()
	refreshClaims := &HGClaims{
		UserID:  userID,
		Device:  device,
		JTI:     refreshJTI,
		TokenTp: "refresh",
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(RefreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "mlc-go",
			Subject:   "user-token",
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(Secret)
	if err != nil {
		return nil, fmt.Errorf("生成 refresh token 失败: %w", err)
	}

	// 存储 Token 状态到 Redis（用于撤销和黑名单）
	accessKey := fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.AuthTokenKey, userID, jti)
	if err := rdb.Set(ctx, accessKey, "1", AccessTTL).Err(); err != nil {
		return nil, fmt.Errorf("存储 access token 失败: %w", err)
	}

	refreshKey := fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.AuthRefreshKey, userID, refreshJTI)
	if err := rdb.Set(ctx, refreshKey, "1", RefreshTTL).Err(); err != nil {
		return nil, fmt.Errorf("存储 refresh token 失败: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
	}, nil
}

// RefreshToken 刷新 Access Token（大厂标准实现）。
// 流程：
//  1. 验证 Refresh Token 签名和有效期
//  2. 检查 Redis 中是否存在（未被撤销）
//  3. 撤销旧的 Refresh Token（一次性使用）
//  4. 生成新的 Token Pair
func RefreshToken(
	ctx context.Context,
	rdb *redis.Client,
	refreshTokenString string,
) (*TokenPair, error) {
	if refreshTokenString == "" {
		return nil, errors.New("refresh token 不能为空")
	}

	// 1. 解析并验证 Refresh Token
	claims := &HGClaims{}
	token, err := jwt.ParseWithClaims(refreshTokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return Secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("refresh token 无效: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("refresh token 已过期")
	}

	// 2. 验证 Token 类型
	if claims.TokenTp != "refresh" {
		return nil, errors.New("token 类型错误，需要 refresh token")
	}

	// 3. 检查 Redis 中是否存在（未被撤销）
	refreshKey := fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.AuthRefreshKey, claims.UserID, claims.JTI)
	exists, err := rdb.Exists(ctx, refreshKey).Result()
	if err != nil {
		return nil, fmt.Errorf("检查 refresh token 状态失败: %w", err)
	}
	if exists == 0 {
		return nil, errors.New("refresh token 已被撤销或不存在")
	}

	// 4. 撤销旧的 Refresh Token（一次性使用，防止重放攻击）
	if err := rdb.Del(ctx, refreshKey).Err(); err != nil {
		return nil, fmt.Errorf("撤销旧 refresh token 失败: %w", err)
	}

	// 5. 撤销旧的 Access Token
	oldAccessKey := fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.AuthTokenKey, claims.UserID, claims.JTI)
	rdb.Del(ctx, oldAccessKey) // 忽略错误，可能已过期

	// 6. 生成新的 Token Pair（继承设备信息和角色）
	return GenerateTokens(ctx, rdb, claims.UserID, claims.Device, claims.Role)
}

// Logout 用户登出，撤销所有 Token。
func Logout(ctx context.Context, rdb *redis.Client, userID string, jti string) error {
	if userID == "" || jti == "" {
		return errors.New("userID 和 jti 不能为空")
	}

	// 撤销 Access Token
	accessKey := fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.AuthTokenKey, userID, jti)
	rdb.Del(ctx, accessKey)

	// 撤销 Refresh Token
	refreshKey := fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.AuthRefreshKey, userID, jti)
	rdb.Del(ctx, refreshKey)

	return nil
}

// LogoutAll 撤销用户的所有 Token（强制下线）。
func LogoutAll(ctx context.Context, rdb *redis.Client, userID string) error {
	if userID == "" {
		return errors.New("userID 不能为空")
	}

	// 删除所有 Access Token
	accessPattern := fmt.Sprintf("%s%s:*", PersistenceRedisPackage.AuthTokenKey, userID)
	keys, _ := rdb.Keys(ctx, accessPattern).Result()
	if len(keys) > 0 {
		rdb.Del(ctx, keys...)
	}

	// 删除所有 Refresh Token
	refreshPattern := fmt.Sprintf("%s%s:*", PersistenceRedisPackage.AuthRefreshKey, userID)
	keys, _ = rdb.Keys(ctx, refreshPattern).Result()
	if len(keys) > 0 {
		rdb.Del(ctx, keys...)
	}

	return nil
}

func (s *HGAuthService) SendCodeV2(ctx context.Context, phone, ip string) error {
	if err := s.limiter.Check(ctx, phone, ip); err != nil {
		return err
	}

	code := UtilsPackage.GenerazteVerifyCode()
	if err := s.sms.Send(phone, code); err != nil {
		return err
	}
	return s.codes.SetCode(ctx, phone, code)
}

// Deprecated: 使用 SendCodeV2 替代, 后面可以删除若是其他地方没有用了
func (s *HGAuthService) SendCode(ctx context.Context, d *UserDtoPackage.SendCodeDTO) error {
	code := "123456" //TODO: 实际用随机值
	return s.codes.SetCode(ctx, d.Phone, code)
}

func (s *HGAuthService) LoginV2(ctx context.Context, phone, code string) (*UserDtoPackage.LoginResultDTO, error) {
	real, err := s.codes.GetCode(ctx, phone)

	if err != nil {
		return nil, errors.New("验证码错误")
	}
	if decodeRedisStringValue(real) != code {
		return nil, errors.New("验证码错误")
	}

	user, _ := s.users.GetByPhone(ctx, phone)
	if user == nil {
		user = &UserModelsPackage.HGUserModel{Phone: UtilsPackage.StrPtrToNullStr(&phone)}
		s.users.Insert(ctx, user)
	}

	tokenPair, err := GenerateTokens(ctx, s.rdb, *UtilsPackage.NullStrToPtr(user.UserID), "", "user")
	if err != nil {
		return nil, fmt.Errorf("生成 token 失败: %w", err)
	}
	s.codes.DeleteCode(ctx, phone)

	return &UserDtoPackage.LoginResultDTO{
		UserID:       user.ID,
		Token:        tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}

// Deprecated: 使用 LoginV2 替代, 后面可以删除若是其他地方没有用了
func (s *HGAuthService) Login(ctx context.Context, d *UserDtoPackage.LoginDTO) (*UserDtoPackage.LoginResultDTO, error) {
	code, err := s.codes.GetCode(ctx, d.Phone)
	if err != nil {
		return nil, errors.New("验证码错误")
	}
	if decodeRedisStringValue(code) != d.Code {
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

/* Token黑名单+ 多端登录控制 */
func (s *HGAuthService) Store(ctx context.Context, uid int64,
	device, jti string, ttl time.Duration) {

	s.codes.SaveMultiportConcrolCache(ctx, uid, device, jti, ttl)
}
func (s *HGAuthService) Valid(ctx context.Context, uid int64,
	deice, jti string) bool {
	v, err := s.codes.GetMultiportConcrolCache(ctx, uid, deice)
	if err != nil {
		logHG.ErrInfo("Token黑名单+ 多端登录控制 cache 获取失败")
		return false
	}
	// 废弃
	// v := s.rdb.Get(ctx,
	// 	fmt.Sprintf("token:%d:%s", uid, deice),
	// ).Val()

	// 是否踢下线
	return v == jti
}

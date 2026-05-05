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

// Token 结构定义
var (
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 7 * 24 * time.Hour
	Secret     = []byte("change-me") //todo：jwt用来签名和验签的对称密钥，token不被修改，通常放在配置中心或者环境变量中，长度 >= 32字节
)

type HGClaims struct {
	UserID  string `json:"uid"`
	Device  string `json:"device"`
	JTI     string `json:"jti"`
	TokenTp string `json:"tp"`
	Role    string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

func NewAuthService(
	users *UserRepositoryPackage.UserRepo,
	codes *UserCachePackage.HGCodeCache,
) *HGAuthService {
	return &HGAuthService{
		users: users,
		codes: codes,
	}
}

// 生成 Access / Refresh Token
func randJTI() string {
	b := make([]byte, 16)
	rand.Read(b)

	return hex.EncodeToString(b)
}

// TODO：鉴权改进，使用中大型公司方案：https://www.qianwen.com/share/chat/6712b6ccfeda4307a0d50a3a7e7c9551
/* 生成Access/Refresh Token */
func GenerateTokens(
	ctx context.Context,
	rdb *redis.Client,
	userID string,
) (access, refresh string, err error) {
	jti := randJTI()
	now := time.Now()

	claims := HGClaims{
		UserID: userID, //表示哪个用户
		JTI:    jti,    // jwt的唯一ID类似UUID，用于防重复攻击
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),                // token签发时间
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)), //token过期时间
		},
	}

	// 创建一个jwt对象，使用签名算法HS256，将claims塞进去。，用迷药Secret对JST签名， at 就是 AccessToken
	at, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(Secret)

	claims.ExpiresAt = jwt.NewNumericDate(now.Add(RefreshTTL))
	rt, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(Secret)

	rdb.Set(
		ctx,
		fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.AuthTokenKey, userID, jti),
		"1",
		AccessTTL,
	)

	rdb.Set(
		ctx,
		fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.AuthRefreshKey, userID, jti),
		"1",
		RefreshTTL,
	)

	return at, rt, nil
}

/* 登录刷新 */
func RefreshToken(
	ctx context.Context,
	rdb *redis.Client,
	refreshToken string,
) (string, error) {
	claims := &HGClaims{}
	// 解析一个refreshToken， t为ture表示签名正确，没有篡改
	t, err := jwt.ParseWithClaims(refreshToken, claims, func(t *jwt.Token) (any, error) {
		// jwt库验证签名时，会调用这个
		// 返回用于验证的密钥
		return Secret, nil
	})
	if err != nil || !t.Valid {
		return "", err
	}

	key := fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.AuthRefreshKey, claims.UserID, claims.JTI)
	if rdb.Exists(ctx, key).Val() == 0 {
		return "", errors.New("refresh token 无效")
	}
	access, _, err := GenerateTokens(ctx, rdb, claims.UserID)
	return access, err
}

/* 推出登录【注销】 */
func Logout(ctx context.Context, rdb *redis.Client, userId int64, jti string) {
	rdb.Del(ctx,
		fmt.Sprintf("%s%d:%s", PersistenceRedisPackage.AuthTokenKey, userId, jti),
		fmt.Sprintf("%s%d:%s", PersistenceRedisPackage.AuthRefreshKey, userId, jti),
	)
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

	at, rt, _ := GenerateTokens(ctx, s.rdb, *UtilsPackage.NullStrToPtr(user.UserID))
	s.codes.DeleteCode(ctx, phone)

	return &UserDtoPackage.LoginResultDTO{
		UserID:       user.ID,
		Token:        at,
		RefreshToken: rt,
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

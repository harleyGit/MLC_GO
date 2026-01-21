/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-21 20:23:05
 * @LastEditors: Harley harelysoa@qq.com
 * @LastEditTime: 2026-01-21 23:08:10
 * @FilePath: /MLC_GO/internal/modules/user/service/hg_auth_service.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserServicePackage

import (
	"MLC_GO/TestNotes/GenPracticeExample/middleware/jwt"
	UserCachePackage "MLC_GO/internal/modules/user/cache"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	UserRepositoryPackage "MLC_GO/internal/modules/user/repository"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type HGAuthService struct {
	users *UserRepositoryPackage.UserRepo
	codes *UserCachePackage.HGCodeCache
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

func (s *HGAuthService) SendCode(ctx context.Context, d *UserDtoPackage.SendCodeDTO) error {
	code := "123456" //TODO: 实际用随机值
	return s.codes.SetCode(ctx, d.Phone, code)
}
func (s *HGAuthService) Login(ctx context.Context, d *UserDtoPackage.LoginDTO) (*UserDtoPackage.LoginResultDTO, error) {
	code, err := s.codes.GetCode(ctx, d.Phone)
	if err != nil || code != d.Code {
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






var AuthCodePhoneLimitKey = "auth:code:limit:phone:"
var AuthCodeIPLimitKey = "auth:code:limit:phone:"
var AuthTokenKey = "auth:token:"
var AuthRefreshKey = "auth:refresh:"
func (r *HGSMSRateLimiter) Check(ctx context.Context, phone, ip string) error {
	phoneKey := fmt.Sprintf("%s%s", AuthCodePhoneLimitKey, phone)
	ipKey := fmt.Sprintf("%s%s", AuthCodeIPLimitKey, ip)

	r.rdb.Expire(ctx, phoneKey, time.Minute)
	r.rdb.Expire(ctx, ipKey, time.Minute)

	if ip.Val() > 5 {
		return errors.New("手机号发送过于频繁")
	}

	if i.Val() > 20 {
		return errors.New("IP 请求过多")
	}
	return nil
}

// Token 结构定义
var(
	AccessTTL = 15 * time.Minute
	RefreshTTL = 7 * 24 *time.Hour
	Secret = []byte("change-me")
)

type HGClaims struct {
	UserID int64 `json:"uid"`
	JTI string `json:"jti"`
	jwt.RegisteredClaims
}



// 生成 Access / Refresh Token
func randJTI() string {
	b := make([]byte, 16)
	rand.Read(b)

	return hex.EncodeToString(b)
}

func GenerateTokens(
	ctx context.Context,
	rdb *redis.Client,
	userID int64,
) (access, refresh string, err error) {
	jti := randJTI()
	now := time.Now()

	claims := Claims{
		UserID: userID,
		JTI: jti,
		RegisteredClaims: jwt.RegisteredClaims {
			IssueAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
		},
	}

	at, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)

	claims.ExpiresAt = jwt.NewNumericDate(now.Add(RefreshTTL))
	rt, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(Secret)

	rdb.Set(
		ctx,
		fmt.Sprintf("%s%d:%s", AuthTokenKey, userID, jti),
		"1",
		AccessTTL,
	)

	rdb.Set(
		ctx,
		fmt.Sprintf("%s%d:%s", AuthRefreshKey, userID, jti),
		"1",
		RefreshTTL,
	)

	return at, rt, nil
}







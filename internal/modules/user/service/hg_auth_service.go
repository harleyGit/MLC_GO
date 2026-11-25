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
	HGSMSPackage "MLC_GO/internal/modules/sms"
	HGSMSCachePackage "MLC_GO/internal/modules/sms/cache"
	usercache "MLC_GO/internal/modules/user/cache"
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	userrepository "MLC_GO/internal/modules/user/repository"
	"MLC_GO/internal/pkg/logHG"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type HGAuthService struct {
	users        *userrepository.UserRepo
	codes        *usercache.HGCodeCache
	limiter      *HGSMSCachePackage.HGSMSRateLimiter
	sms          *HGSMSPackage.HGPhoneSMSSender
	redisService *PersistenceRedisPackage.RedisService
	rdb          *redis.Client
}

// hgSessionDB 是当前服务实例的 IM 强制下线权威库；初始化后原子发布，未配置时新会话鉴权失败关闭。
var hgSessionDB atomic.Pointer[sql.DB]

// HGConfigureSessionDB 在注册用户路由前设置强制下线查询所用 MySQL 连接池。
func HGConfigureSessionDB(db *sql.DB) { hgSessionDB.Store(db) }

// HGCheckAuthRecoverySchema 监听前校验迁移完成，防止缺表时应用启动后大面积拒绝已有会话。
func HGCheckAuthRecoverySchema(ctx context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, SQLQueriesPackage.HGCheckRecoverySchema)
	if err != nil {
		return fmt.Errorf("无法读取 MFA 恢复表，请检查迁移 000087 及数据库权限: %w", err)
	}
	return rows.Close()
}

// NewAuthService 创建认证服务。
func NewAuthService(
	users *userrepository.UserRepo,
	codes *usercache.HGCodeCache,
	redisService *PersistenceRedisPackage.RedisService,
) *HGAuthService {
	var rdb *redis.Client
	if redisService != nil {
		rdb = redisService.Client()
	}

	return &HGAuthService{
		users:        users,
		codes:        codes,
		redisService: redisService,
		rdb:          rdb,
	}
}

// Token 结构定义
var (
	AccessTTL  = 24 * time.Hour
	RefreshTTL = 30 * 24 * time.Hour
	Secret     = hgEphemeralJWTSecret() // 仅 debug 未配置时使用进程内随机值；pre/prod 启动须注入稳定密钥。
)

// HGClaims JWT Claims 结构。
type HGClaims struct {
	UserID    string `json:"uid"`            // 用户 ID
	SessionID string `json:"sid,omitempty"`  // 同一登录会话的 access/refresh 共用；旧 token 为空。
	Device    string `json:"device"`         // 设备信息（支持多端登录控制）
	JTI       string `json:"jti"`            // JWT 唯一 ID（防重放攻击）
	TokenTp   string `json:"tp"`             // Token 类型：access / refresh
	Role      string `json:"role,omitempty"` // 用户角色
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
	generation, err := hgUserSessionGeneration(ctx, userID)
	if err != nil {
		return nil, err
	}
	return hgNewSessionTokens(ctx, rdb, userID, device, role, generation)
}

func hgUserSessionGeneration(ctx context.Context, userID string) (uint64, error) {
	db := hgSessionDB.Load()
	if db == nil || userID == "" {
		return 0, errors.New("无法读取账号会话代际")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var generation uint64
	err := db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectRecoveryGeneration, userID).Scan(&generation)
	return generation, err
}

func hgNewSessionTokens(ctx context.Context, rdb *redis.Client, userID, device, role string, generation uint64) (*TokenPair, error) {
	if userID == "" || rdb == nil {
		return nil, errors.New("登录用户或 Redis 不可用")
	}
	var hgSessionBytes [16]byte
	if _, err := rand.Read(hgSessionBytes[:]); err != nil {
		return nil, fmt.Errorf("生成登录会话 ID 失败: %w", err)
	}
	sessionID := hex.EncodeToString(hgSessionBytes[:])
	return hgGenerateSessionTokens(ctx, rdb, userID, device, role, sessionID, generation)
}

// hgGenerateSessionTokens 首次签发并原子登记会话；后续刷新绝不调用此函数重建 refresh。
func hgGenerateSessionTokens(ctx context.Context, rdb *redis.Client, userID, device, role, sessionID string, generation uint64) (*TokenPair, error) {
	if deleted, err := HGUserDeleted(ctx, rdb, userID); err != nil || deleted {
		return nil, errors.New("账号不可登录")
	}

	if role == "" {
		role = "user"
	}

	now := time.Now()
	accessTokenString, err := hgSignAccessToken(userID, device, role, sessionID, now)
	if err != nil {
		return nil, err
	}
	refreshJTI := randJTI()
	// refresh JWT 不设置固定 exp；签名和服务端 Redis TTL 共同限定有效性，过期后绝不能重建。
	refreshClaims := &HGClaims{
		UserID:    userID,
		SessionID: sessionID,
		Device:    device,
		JTI:       refreshJTI,
		TokenTp:   "refresh",
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "mlc-go",
			Subject:   "user-token",
		},
	}
	refreshTokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(Secret)
	if err != nil {
		return nil, fmt.Errorf("生成 refresh token 失败: %w", err)
	}
	keys := hgAuthSessionKeys(userID, sessionID, refreshJTI)
	ok, err := rdb.Eval(ctx, PersistenceRedisPackage.HGAuthRegisterSessionLuaScript, keys,
		sessionID, refreshJTI, int(RefreshTTL.Seconds()), PersistenceRedisPackage.HGAuthSessionKey+userID+":",
		PersistenceRedisPackage.AuthRefreshKey+userID+":", generation,
		PersistenceRedisPackage.HGAuthSessionProofKey+userID+":").Int()
	if err != nil {
		return nil, fmt.Errorf("存储登录会话失败: %w", err)
	}
	if ok != 1 {
		return nil, errors.New("账号不可登录")
	}
	return &TokenPair{AccessToken: accessTokenString, RefreshToken: refreshTokenString}, nil
}

// hgSignAccessToken 每次签发独立 JTI；刷新不修改此前的 access token 或会话状态。
func hgSignAccessToken(userID, device, role, sessionID string, now time.Time) (string, error) {
	accessClaims := &HGClaims{
		UserID:    userID,
		SessionID: sessionID,
		Device:    device,
		JTI:       randJTI(),
		TokenTp:   "access",
		Role:      role,
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
		return "", fmt.Errorf("生成 access token 失败: %w", err)
	}
	return accessTokenString, nil
}

// hgAuthSessionKeys 给登录与续签 Lua 提供同一组 key；均由服务端 JWT 主体构造。
func hgAuthSessionKeys(userID, sessionID, refreshJTI string) []string {
	return []string{
		PersistenceRedisPackage.HGAuthSessionIndexKey + userID,
		PersistenceRedisPackage.HGAuthSessionKey + userID + ":" + sessionID,
		PersistenceRedisPackage.AuthRefreshKey + userID + ":" + refreshJTI,
		PersistenceRedisPackage.HGUserDeleteFenceKey(userID),
		PersistenceRedisPackage.HGAuthSessionProofKey + userID + ":" + sessionID,
	}
}

// RefreshToken 使用同一 refresh JWT 滑动续期；Redis 负责 30 天闲置过期及会话撤销。
func RefreshToken(
	ctx context.Context,
	rdb *redis.Client,
	refreshTokenString string,
) (*TokenPair, error) {
	if refreshTokenString == "" {
		return nil, errors.New("refresh token 不能为空")
	}
	if rdb == nil {
		return nil, errors.New("Redis 不可用")
	}

	// 旧版 JWT 含固定 7 天 exp；只有服务端状态仍存在时才允许其跨越原 exp 继续续签。
	claims := &HGClaims{}
	token, err := jwt.ParseWithClaims(refreshTokenString, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return Secret, nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		return nil, fmt.Errorf("refresh token 无效: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("refresh token 签名无效")
	}
	now := time.Now()
	if claims.TokenTp != "refresh" || claims.UserID == "" || claims.JTI == "" ||
		claims.Issuer != "mlc-go" || claims.Subject != "user-token" ||
		claims.IssuedAt == nil || claims.IssuedAt.Time.After(now.Add(time.Minute)) ||
		(claims.NotBefore != nil && claims.NotBefore.Time.After(now)) {
		return nil, errors.New("refresh token 声明无效")
	}
	if claims.SessionID == "" {
		return nil, errors.New("旧版无会话令牌须重新登录")
	}
	active, err := HGSessionActive(ctx, rdb, claims.UserID, claims.SessionID)
	if err != nil || !active {
		return nil, errors.New("登录会话已失效")
	}
	accessTokenString, err := hgSignAccessToken(claims.UserID, claims.Device, claims.Role, claims.SessionID, now)
	if err != nil {
		return nil, err
	}
	keys := hgAuthSessionKeys(claims.UserID, claims.SessionID, claims.JTI)
	ok, err := rdb.Eval(ctx, PersistenceRedisPackage.HGAuthRefreshSessionLuaScript, keys,
		claims.SessionID, claims.JTI, int(RefreshTTL.Seconds()), PersistenceRedisPackage.HGAuthSessionKey+claims.UserID+":",
		PersistenceRedisPackage.AuthRefreshKey+claims.UserID+":", 0,
		PersistenceRedisPackage.HGAuthSessionProofKey+claims.UserID+":").Int()
	if err != nil {
		return nil, fmt.Errorf("续期登录会话失败: %w", err)
	}
	if ok != 1 {
		return nil, errors.New("refresh token 已过期或会话已撤销")
	}
	return &TokenPair{AccessToken: accessTokenString, RefreshToken: refreshTokenString}, nil
}

// HGSessionActive 校验新 JWT 的会话存活状态；Redis 异常必须失败关闭。
func HGSessionActive(ctx context.Context, rdb *redis.Client, userID, sessionID string) (bool, error) {
	_, active, err := HGActiveSessionGeneration(ctx, rdb, userID, sessionID)
	return active, err
}

// HGActiveSessionGeneration 核对权威代际；零代兼容上线前签发且 Redis 会话仍存活的令牌。
func HGActiveSessionGeneration(ctx context.Context, rdb *redis.Client, userID, sessionID string) (uint64, bool, error) {
	decoded, decodeErr := hex.DecodeString(sessionID)
	db := hgSessionDB.Load()
	if rdb == nil || db == nil || userID == "" || decodeErr != nil || len(decoded) != 16 {
		return 0, false, errors.New("登录会话参数无效")
	}
	n, err := rdb.Exists(ctx, PersistenceRedisPackage.HGAuthSessionKey+userID+":"+sessionID).Result()
	if err != nil || n != 1 {
		return 0, false, err
	}
	checkCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var generation uint64
	var forced bool
	err = db.QueryRowContext(checkCtx, SQLQueriesPackage.HGSelectAuthSessionAuthority, userID, decoded, userID).Scan(&generation, &forced)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil || forced {
		return 0, false, err
	}
	if generation != 0 {
		proof, err := rdb.Get(checkCtx, PersistenceRedisPackage.HGAuthSessionProofKey+userID+":"+sessionID).Uint64()
		if errors.Is(err, redis.Nil) {
			return 0, false, nil
		}
		if err != nil || proof != generation {
			return 0, false, err
		}
	}
	return generation, true, nil
}

type hgSessionGenerationContextKey struct{}

// HGContextWithSessionGeneration 仅由通过鉴权的中间件记录服务端会话代际，不读取客户端数据。
func HGContextWithSessionGeneration(ctx context.Context, generation uint64) context.Context {
	return context.WithValue(ctx, hgSessionGenerationContextKey{}, generation)
}

// HGSessionGenerationFromContext 把已鉴权的代际交给加锁写操作复核，避免旧请求跨越恢复事务。
func HGSessionGenerationFromContext(ctx context.Context) (uint64, bool) {
	value, ok := ctx.Value(hgSessionGenerationContextKey{}).(uint64)
	return value, ok
}

// HGRevokeAuthSession 撤销当前会话、索引和对应 refresh 状态；失败时不能声称登出成功。
func HGRevokeAuthSession(ctx context.Context, rdb *redis.Client, userID, sessionID string) error {
	decoded, err := hex.DecodeString(sessionID)
	if rdb == nil || userID == "" || err != nil || len(decoded) != 16 {
		return errors.New("登录会话参数无效")
	}
	return rdb.Eval(ctx, PersistenceRedisPackage.HGAuthRevokeSessionLuaScript,
		[]string{PersistenceRedisPackage.HGAuthSessionIndexKey + userID,
			PersistenceRedisPackage.HGAuthSessionKey + userID + ":" + sessionID,
			PersistenceRedisPackage.HGAuthSessionProofKey + userID + ":" + sessionID},
		sessionID, PersistenceRedisPackage.AuthRefreshKey+userID+":").Err()
}

// RefreshToken 通过注入的 Redis 客户端刷新 Token，避免 handler 直接依赖全局 RDB。
// 业务规则沿用包级 RefreshToken：服务端状态续期并签发新 access，原 refresh 不轮换。
func (s *HGAuthService) RefreshToken(ctx context.Context, refreshTokenString string) (*TokenPair, error) {
	if s == nil || s.rdb == nil {
		return nil, errors.New("auth service redis dependency is nil")
	}
	return RefreshToken(ctx, s.rdb, refreshTokenString)
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
	if DecodeRedisStringValue(real) != code {
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
	if err := s.codes.DeleteCode(ctx, phone); err != nil {
		logHG.DebugFInfo("DeleteCode err: %v", err)
	}

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
	if DecodeRedisStringValue(code) != d.Code {
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

	if err := s.codes.DeleteCode(ctx, d.Phone); err != nil {
		logHG.DebugFInfo("DeleteCode err: %v", err)
	}

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

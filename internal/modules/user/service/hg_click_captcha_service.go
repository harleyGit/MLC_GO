/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-05-31
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-31
 * @FilePath: /MLC_GO/internal/modules/user/service/hg_click_captcha_service.go
 * @Description: 点选验证码服务，生成图片验证码并验证用户点选结果
 */
package UserServicePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// 点选验证码相关错误
var (
	ErrCaptchaNotFound = errors.New("验证码不存在或已过期")
	ErrCaptchaInvalid  = errors.New("验证码验证失败")
)

// ClickCaptchaPoint 表示验证码图片上的一个点位。
type ClickCaptchaPoint struct {
	X int    `json:"x"` // 点位在图片上的 X 坐标
	Y int    `json:"y"` // 点位在图片上的 Y 坐标
}

// ClickCaptchaResponse 是获取点选验证码时返回的数据。
type ClickCaptchaResponse struct {
	CaptchaID string   `json:"captchaId"` // 验证码唯一标识
	ImageURL  string   `json:"imageUrl"`  // 验证码图片 URL（Base64 或实际 URL）
	Chars     []string `json:"chars"`     // 需要按顺序点选的字符
}

// ClickCaptchaVerifyRequest 是验证点选结果的请求。
type ClickCaptchaVerifyRequest struct {
	CaptchaID string               `json:"captchaId"` // 验证码唯一标识
	Points    []ClickCaptchaPoint  `json:"points"`    // 用户点击的坐标序列
}

// ClickCaptchaVerifyResponse 是验证点选结果的响应。
type ClickCaptchaVerifyResponse struct {
	Valid       bool   `json:"valid"`       // 验证是否通过
	VerifyToken string `json:"verifyToken"` // 验证通过后的 token，用于后续发送验证码
}

// CaptchaData 存储在 Redis 中的验证码数据。
type CaptchaData struct {
	Chars   []string           `json:"chars"`   // 验证码字符序列
	Points  []ClickCaptchaPoint `json:"points"`  // 字符在图片上的位置
	Created int64              `json:"created"`  // 创建时间戳
}

const (
	// 点选验证码 Redis key 前缀
	clickCaptchaKeyPrefix = "auth:click_captcha:"
	// 验证码有效期
	clickCaptchaTTL = 5 * time.Minute
	// 验证通过后的 token 前缀
	clickCaptchaTokenPrefix = "auth:click_captcha_token:"
	// 验证 token 有效期
	clickCaptchaTokenTTL = 10 * time.Minute
	// 点击容差范围（像素）
	clickTolerance = 30
)

// ClickCaptchaService 点选验证码服务。
type ClickCaptchaService struct {
	redisService *PersistenceRedisPackage.RedisService
}

// NewClickCaptchaService 创建点选验证码服务。
func NewClickCaptchaService(redisService *PersistenceRedisPackage.RedisService) *ClickCaptchaService {
	return &ClickCaptchaService{
		redisService: redisService,
	}
}

// GenerateCaptcha 生成点选验证码。
// 返回验证码 ID、图片 Base64 数据和需要点选的字符。
func (s *ClickCaptchaService) GenerateCaptcha(ctx context.Context) (*ClickCaptchaResponse, error) {
	if s == nil || s.redisService == nil {
		return nil, fmt.Errorf("captcha service not initialized")
	}

	// 生成 3-5 个随机字符（数字、字母、汉字混合）
	charCount := 3 + rand.Intn(3) // 3-5 个字符
	chars := generateRandomChars(charCount)

	// 生成字符在图片上的随机位置
	points := generateRandomPoints(charCount, 280, 100) // 假设图片宽度 280，高度 100

	// 生成唯一 ID
	captchaID := generateCaptchaID()

	// 构建验证码数据
	captchaData := CaptchaData{
		Chars:   chars,
		Points:  points,
		Created: time.Now().UnixMilli(),
	}

	// 存储到 Redis
	key := clickCaptchaKeyPrefix + captchaID
	dataBytes, err := json.Marshal(captchaData)
	if err != nil {
		return nil, fmt.Errorf("marshal captcha data: %w", err)
	}

	if err := s.redisService.SetToRedisV2(key, string(dataBytes), clickCaptchaTTL, ctx); err != nil {
		return nil, fmt.Errorf("save captcha to redis: %w", err)
	}

	// 生成图片 Base64
	imageBase64, err := generateCaptchaImage(chars, points)
	if err != nil {
		return nil, fmt.Errorf("generate captcha image: %w", err)
	}

	return &ClickCaptchaResponse{
		CaptchaID: captchaID,
		ImageURL:  "data:image/png;base64," + imageBase64,
		Chars:     chars,
	}, nil
}

// VerifyCaptcha 验证点选结果。
// 返回验证是否通过和验证 token。
func (s *ClickCaptchaService) VerifyCaptcha(ctx context.Context, req *ClickCaptchaVerifyRequest) (*ClickCaptchaVerifyResponse, error) {
	if s == nil || s.redisService == nil {
		return nil, fmt.Errorf("captcha service not initialized")
	}

	// 从 Redis 获取验证码数据
	key := clickCaptchaKeyPrefix + req.CaptchaID
	dataStr, err := s.redisService.GetFromRedisV2(key, ctx)
	if err != nil {
		return nil, fmt.Errorf("get captcha from redis: %w", err)
	}

	if dataStr == "" {
		return &ClickCaptchaVerifyResponse{Valid: false}, ErrCaptchaNotFound
	}

	// 解析验证码数据
	var captchaData CaptchaData
	if err := json.Unmarshal([]byte(dataStr), &captchaData); err != nil {
		return nil, fmt.Errorf("unmarshal captcha data: %w", err)
	}

	// 验证点选结果
	if !verifyClickPoints(captchaData.Points, req.Points) {
		return &ClickCaptchaVerifyResponse{Valid: false}, ErrCaptchaInvalid
	}

	// 验证通过，删除已使用的验证码
	s.redisService.DeleteFromRedis(key, ctx)

	// 生成验证 token
	verifyToken := generateCaptchaID()
	tokenKey := clickCaptchaTokenPrefix + verifyToken
	if err := s.redisService.SetToRedisV2(tokenKey, "1", clickCaptchaTokenTTL, ctx); err != nil {
		return nil, fmt.Errorf("save verify token: %w", err)
	}

	return &ClickCaptchaVerifyResponse{
		Valid:       true,
		VerifyToken: verifyToken,
	}, nil
}

// ValidateVerifyToken 验证点选验证码的 token 是否有效。
func (s *ClickCaptchaService) ValidateVerifyToken(ctx context.Context, verifyToken string) (bool, error) {
	if s == nil || s.redisService == nil {
		return false, fmt.Errorf("captcha service not initialized")
	}

	if verifyToken == "" {
		return false, nil
	}

	key := clickCaptchaTokenPrefix + verifyToken
	val, err := s.redisService.GetFromRedisV2(key, ctx)
	if err != nil {
		return false, fmt.Errorf("get verify token from redis: %w", err)
	}

	if val == "" {
		return false, nil
	}

	// 验证通过后删除 token（一次性使用）
	s.redisService.DeleteFromRedis(key, ctx)

	return true, nil
}

// verifyClickPoints 验证用户点击的坐标是否在容差范围内。
func verifyClickPoints(expected []ClickCaptchaPoint, actual []ClickCaptchaPoint) bool {
	if len(expected) != len(actual) {
		return false
	}

	for i, exp := range expected {
		act := actual[i]
		// 计算两点之间的距离
		dx := exp.X - act.X
		dy := exp.Y - act.Y
		if dx*dx+dy*dy > clickTolerance*clickTolerance {
			return false
		}
	}

	return true
}

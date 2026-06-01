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
	X int `json:"x"` // 点位在图片上的 X 坐标
	Y int `json:"y"` // 点位在图片上的 Y 坐标
}

// ClickCaptchaResponse 是获取点选验证码时返回的数据。
type ClickCaptchaResponse struct {
	CaptchaID string   `json:"captchaId"` // 验证码唯一标识
	ImageURL  string   `json:"imageUrl"`  // 验证码图片 URL（Base64 或实际 URL）
	Chars     []string `json:"chars"`     // 需要按顺序点选的字符
}

// ClickCaptchaVerifyRequest 是验证点选结果的请求。
type ClickCaptchaVerifyRequest struct {
	CaptchaID string              `json:"captchaId"` // 验证码唯一标识
	Points    []ClickCaptchaPoint `json:"points"`    // 用户点击的坐标序列
}

// ClickCaptchaVerifyResponse 是验证点选结果的响应。
type ClickCaptchaVerifyResponse struct {
	Valid       bool   `json:"valid"`       // 验证是否通过
	VerifyToken string `json:"verifyToken"` // 验证通过后的 token，用于后续发送验证码
}

// CaptchaData 存储在 Redis 中的验证码数据。
type CaptchaData struct {
	Chars   []string            `json:"chars"`   // 验证码字符序列
	Points  []ClickCaptchaPoint `json:"points"`  // 字符在图片上的位置
	Created int64               `json:"created"` // 创建时间戳
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

	// 生成 3-5 个随机字符（数字、字母、汉字混合）。
	// 字符数量太少会降低验证码强度，太多会增加用户操作成本；3-5 个字符是交互成本和安全性的折中。
	charCount := 3 + rand.Intn(3) // 3-5 个字符
	chars := generateRandomChars(charCount)

	// 生成字符在图片上的随机位置。
	// 这里的坐标是“原始图片坐标系”，当前图片生成函数固定输出 280x100。
	// 前端页面可能缩放展示图片，因此前端提交点击坐标前必须换算回这个坐标系。
	points := generateRandomPoints(charCount, 280, 100) // 假设图片宽度 280，高度 100

	// 生成唯一 ID
	captchaID := generateCaptchaID()

	// 构建验证码数据。
	// Chars 返回给前端用于提示“按顺序点击哪些字符”；Points 只存 Redis，不返回前端。
	// VerifyCaptcha 后续会拿用户点击点与 Points 做容差匹配，防止前端伪造直接通过。
	captchaData := CaptchaData{
		Chars:   chars,
		Points:  points,
		Created: time.Now().UnixMilli(),
	}

	// 存储到 Redis。SetToRedisV2 会统一 JSON 序列化，直接传结构体避免二次编码成 JSON 字符串。
	// 错误示例：先 json.Marshal(captchaData) 再 string(dataBytes) 传入 SetToRedisV2。
	// 这样 Redis 中会保存成 JSON string，VerifyCaptcha 再反序列化 CaptchaData 会报：
	// json: cannot unmarshal string into Go value of type UserServicePackage.CaptchaData。
	key := clickCaptchaKeyPrefix + captchaID
	if err := s.redisService.SetToRedisV2(key, captchaData, clickCaptchaTTL, ctx); err != nil {
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

	// 从 Redis 获取验证码数据。
	// captchaID 是前端从 /click_captcha 拿到的随机 ID，Redis key 中只拼 ID，不暴露真实字符坐标。
	key := clickCaptchaKeyPrefix + req.CaptchaID
	dataStr, err := s.redisService.GetFromRedisV2(key, ctx)
	if err != nil {
		return nil, fmt.Errorf("get captcha from redis: %w", err)
	}

	if dataStr == "" {
		return &ClickCaptchaVerifyResponse{Valid: false}, ErrCaptchaNotFound
	}

	// 解析验证码数据。
	// parseCaptchaData 同时支持新格式（JSON object）和旧格式（JSON encoded string），避免未过期旧 key 造成线上验证失败。
	captchaData, err := parseCaptchaData(dataStr)
	if err != nil {
		return nil, fmt.Errorf("unmarshal captcha data: %w", err)
	}

	// 验证点选结果。
	// actual 点位必须和 expected 点位顺序一致；点选验证码的安全性来自“按提示顺序点击”，不是只要点到字符即可。
	if !verifyClickPoints(captchaData.Points, req.Points) {
		return &ClickCaptchaVerifyResponse{Valid: false}, ErrCaptchaInvalid
	}

	// 验证通过，删除已使用的验证码。
	// 这一步保证同一个 captchaID 只能被验证一次，避免用户或脚本复用同一张图片反复换取 verifyToken。
	s.redisService.DeleteFromRedis(key, ctx)

	// 生成验证 token。
	// verifyToken 是给 send_code / send_email_code 使用的一次性凭证，不等同于登录 token。
	// 它的 TTL 比图片验证码略长，允许用户点选成功后有时间完成后续“发送验证码”请求。
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

	// token key 与 captcha key 分开前缀，避免图片验证码数据和验证通过凭证混用。
	// 这里只关心 token 是否存在，不需要解析 value；value 当前固定为 "1"。
	key := clickCaptchaTokenPrefix + verifyToken
	val, err := s.redisService.GetFromRedisV2(key, ctx)
	if err != nil {
		return false, fmt.Errorf("get verify token from redis: %w", err)
	}

	if val == "" {
		return false, nil
	}

	// 验证通过后删除 token（一次性使用）。
	// 在高并发下，这里后续可进一步收敛为 Redis Lua：GET+DEL 原子执行，避免同一 token 极短时间内并发双花。
	// 当前逻辑已经满足普通注册发送验证码流程；如果该接口成为热点，应升级为 Lua 原子消费。
	s.redisService.DeleteFromRedis(key, ctx)

	return true, nil
}

// parseCaptchaData 解析 Redis 中保存的点选验证码数据。
// 当前正确格式：SetToRedisV2 直接保存 CaptchaData 结构体，Redis value 是 JSON object。
// 旧兼容格式：历史代码曾经先 json.Marshal(CaptchaData) 得到 JSON bytes，再转 string 交给 SetToRedisV2。
// 因为 SetToRedisV2 内部会再次 json.Marshal，Redis value 就变成 JSON encoded string。
// 保留双格式解析可以让已写入 Redis、仍在 TTL 内的旧验证码继续可用，避免用户正在操作时突然全部失败。
func parseCaptchaData(dataStr string) (CaptchaData, error) {
	var captchaData CaptchaData
	// 优先按新格式解析：{"chars":[...],"points":[...],"created":...}
	if err := json.Unmarshal([]byte(dataStr), &captchaData); err == nil {
		return captchaData, nil
	}

	// 兼容旧数据：历史代码先 Marshal 结构体再以 string 调用 SetToRedisV2，Redis 中会保存成 JSON 字符串。
	var encoded string
	// 旧格式第一层解析得到普通字符串："{\"chars\":[...]}" -> {"chars":[...]}
	if err := json.Unmarshal([]byte(dataStr), &encoded); err != nil {
		return CaptchaData{}, err
	}
	// 再把字符串内容按 CaptchaData 解析。
	if err := json.Unmarshal([]byte(encoded), &captchaData); err != nil {
		return CaptchaData{}, err
	}

	return captchaData, nil
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

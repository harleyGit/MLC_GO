package UserHandlerPackage

import (
	UserService "MLC_GO/internal/modules/user/service"
	Mail "MLC_GO/internal/pkg/email"
	Redis "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
)

type hgEmailCaptchaHook struct{ used bool }

func (*hgEmailCaptchaHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (*hgEmailCaptchaHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *hgEmailCaptchaHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		switch cmd.Name() {
		case "del":
			if !h.used {
				cmd.(*redis.IntCmd).SetVal(1)
			}
			h.used = true
		case "eval":
			cmd.(*redis.Cmd).SetVal(int64(1))
		default:
			return errors.New("测试未预期的 Redis 请求")
		}
		return nil
	}
}

type hgEmailHandlerSender func(context.Context, string, string) error

func (s hgEmailHandlerSender) SendCode(ctx context.Context, address, code string) error {
	return s(ctx, address, code)
}

func TestHGRegistrationEmailEndpointNeverReturnsCode(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		t.Run(method, func(t *testing.T) {
			client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
			defer client.Close()
			client.AddHook(&hgEmailCaptchaHook{})
			cache := Redis.HGNewRedisServiceWithClient(client)
			var sent int
			h := NewUserHandler(HGUserHandlerDeps{
				UserService:     UserService.NewUserService(nil, nil, cache),
				ClickCaptchaSvc: UserService.NewClickCaptchaService(cache),
				EmailSender: hgEmailHandlerSender(func(_ context.Context, address, code string) error {
					if address != "test@example.org" || !Mail.HGValidCode(code) {
						t.Fatal("发信入参错误")
					}
					sent++
					return nil
				}),
			})
			path, body := "/send_email_code", `{"email":"test@example.org","verifyToken":"fixture-captcha"}`
			if method == http.MethodGet {
				path, body = path+"?email=test%40example.org&verifyToken=fixture-captcha", ""
			}
			rec := httptest.NewRecorder()
			h.SendEmailCode(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
			var result struct {
				Code int            `json:"code"`   // 统一响应业务码。
				Data map[string]any `json:"result"` // 成功时仅含发送说明。
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Code != 200 || sent != 1 ||
				len(result.Data) != 1 || result.Data["message"] != "验证码已发送" || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("成功响应不应含验证码或收件人: HTTP %d, 业务码 %d", rec.Code, result.Code)
			}
			h.SendEmailCode(httptest.NewRecorder(), httptest.NewRequest(method, path, strings.NewReader(body)))
			if sent != 1 {
				t.Fatal("旧点选验证凭证不应再次发信")
			}
		})
	}
}

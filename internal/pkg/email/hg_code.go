package hgemail

import (
	Redis "MLC_GO/internal/pkg/redis"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"

	"github.com/redis/go-redis/v9"
)

var (
	HGErrInvalidAddress = errors.New("请输入有效的邮箱地址")
	HGErrRateLimited    = errors.New("邮件发送过于频繁，请稍后再试")
	HGErrUnavailable    = errors.New("邮件服务暂不可用，请稍后重试")
)

// HGNormalizeAddress 接受单一 ASCII 邮箱，包括 Gmail、iCloud 和 QQ；保留本地部分，规范化域名。
func HGNormalizeAddress(value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", HGErrInvalidAddress
	}
	value = strings.TrimSpace(value)
	for _, ch := range value {
		if ch <= ' ' || ch >= 127 || strings.ContainsRune(`<>,()"\`, ch) {
			return "", HGErrInvalidAddress
		}
	}
	local, domain, ok := strings.Cut(value, "@")
	if !ok || local == "" || len(local) > 64 || len(value) > 254 || !strings.Contains(domain, ".") {
		return "", HGErrInvalidAddress
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", HGErrInvalidAddress
		}
		for _, ch := range strings.ToLower(label) {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return "", HGErrInvalidAddress
			}
		}
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return "", HGErrInvalidAddress
	}
	return local + "@" + strings.ToLower(domain), nil
}

// HGNewCode 从密码学随机源无偏生成六位数字（含前导零）。
func HGNewCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HGValidCode 验证六位 ASCII 数字格式，不作隐式数字转换。
func HGValidCode(code string) bool {
	return len(code) == 6 && strings.IndexFunc(code, func(ch rune) bool { return ch < '0' || ch > '9' }) < 0
}

// HGAdmitCodeSend 共享发信配额：每地址每分钟一次、全站每分钟最多 120 次；Redis 不可用时不发信。
func HGAdmitCodeSend(ctx context.Context, client *redis.Client, address string) error {
	count, err := client.Eval(ctx, Redis.HGFixedWindowCountLua, []string{Redis.HGEmailSendLimitKey(address)}, 60, 1).Int()
	if err != nil {
		return HGErrUnavailable
	}
	if count != 1 {
		return HGErrRateLimited
	}
	count, err = client.Eval(ctx, Redis.HGFixedWindowCountLua, []string{Redis.HGEmailGlobalSendLimitKey}, 60, 120).Int()
	if err != nil {
		return HGErrUnavailable
	}
	if count != 1 {
		return HGErrRateLimited
	}
	return nil
}

package hgemail

import (
	"MLC_GO/internal/pkg/logHG"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

// HGSender 发送一次性邮箱验证码；实现不得记录邮件内容或收件人。
type HGSender interface {
	SendCode(ctx context.Context, address, code string) error
}

// HGErrSenderConfig 表示发件服务尚不可用，修正配置后重启进程；不包含任何配置值。
var HGErrSenderConfig = errors.New("邮件发件服务未配置或配置有误，请联系管理员")

// HGCheckCodeSender 在占用发信配额前校验 SMTP 配置；替换发件实现可按需提供配置检查。
// 此检查不会连接 SMTP 或发送邮件。
func HGCheckCodeSender(sender HGSender) error {
	if sender == nil {
		return HGErrSenderConfig
	}
	if configured, ok := sender.(interface{ CheckConfig() error }); ok {
		return hgSafeSenderError(configured.CheckConfig())
	}
	return nil
}

// HGDeliverCode 只向业务层返回受控错误，避免替换发件实现泄露供应商响应或凭据。
func HGDeliverCode(ctx context.Context, sender HGSender, address, code string) error {
	return hgSafeSenderError(sender.SendCode(ctx, address, code))
}

func hgSafeSenderError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, HGErrSenderConfig):
		return HGErrSenderConfig
	case errors.Is(err, HGErrRateLimited):
		return HGErrRateLimited
	default:
		return HGErrUnavailable
	}
}

// HGSMTPSender 从进程环境读取 SMTP 配置；没有配置时拒绝发送，不在代码中存储凭据。
type HGSMTPSender struct{}

var hgSendSlots = make(chan struct{}, 8) // 单实例最多八个 SMTP 连接，满载立即拒绝。

type hgSMTPConfig struct {
	host     string // TLS 证书校验主机名。
	port     string // TLS 邮件提交端口。
	from     string // 配置的信封发件地址。
	user     string // SMTP 认证用户名。
	password string // 部署时注入的应用密码或 SMTP 授权码。
	implicit bool   // true 时先 TLS 后 SMTP；false 时要求 STARTTLS。
}

// NewHGSMTPSender 提供只在实际发送时读取配置的 SMTP 适配器。
func NewHGSMTPSender() *HGSMTPSender { return &HGSMTPSender{} }

// CheckConfig 检查进程内 SMTP 配置，不触发网络访问。
func (s *HGSMTPSender) CheckConfig() error {
	_, err := hgReadSMTPConfig()
	return err
}

func hgReadSMTPConfig() (hgSMTPConfig, error) {
	cfg := hgSMTPConfig{
		host: strings.TrimSpace(os.Getenv("MLC_SMTP_HOST")), port: strings.TrimSpace(os.Getenv("MLC_SMTP_PORT")),
		user: os.Getenv("MLC_SMTP_USER"), password: os.Getenv("MLC_SMTP_PASSWORD"),
	}
	mode := strings.TrimSpace(os.Getenv("MLC_SMTP_TLS_MODE"))
	cfg.implicit = mode == "implicit" || (mode == "" && cfg.port == "465")
	if mode != "" && mode != "starttls" && mode != "implicit" {
		return hgSMTPConfig{}, HGErrSenderConfig
	}
	if cfg.port == "" {
		cfg.port = "587"
		if cfg.implicit {
			cfg.port = "465"
		}
	}
	var err error
	cfg.from, err = HGNormalizeAddress(os.Getenv("MLC_SMTP_FROM"))
	if err != nil || cfg.host == "" || strings.ContainsAny(cfg.host, " /\\\x00\r\n\t") ||
		strings.TrimSpace(cfg.user) == "" || strings.TrimSpace(cfg.password) == "" || strings.ContainsAny(cfg.user+cfg.password, "\x00\r\n") {
		return hgSMTPConfig{}, HGErrSenderConfig
	}
	portNum, err := strconv.Atoi(cfg.port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return hgSMTPConfig{}, HGErrSenderConfig
	}
	return cfg, nil
}

// SendCode 使用证书校验的 STARTTLS 或隐式 TLS；只记录代码内定义的失败阶段。
func (s *HGSMTPSender) SendCode(ctx context.Context, address, code string) error {
	cfg, err := hgReadSMTPConfig()
	if err != nil {
		return err
	}
	address, err = HGNormalizeAddress(address)
	if err != nil || !HGValidCode(code) {
		return errors.New("邮箱验证码参数无效")
	}
	select {
	case hgSendSlots <- struct{}{}:
		defer func() { <-hgSendSlots }()
	default:
		return HGErrRateLimited
	}
	if err := hgSendSMTP(ctx, cfg, &tls.Config{ServerName: cfg.host, MinVersion: tls.VersionTLS12}, address, code); err != nil {
		logHG.ErrFInfo("SMTP 投递失败：%v", err) // hgSendSMTP 的错误均为常量阶段描述，不带第三方响应、地址或配置值。
		return HGErrUnavailable
	}
	return nil
}

func hgSendSMTP(ctx context.Context, cfg hgSMTPConfig, tlsConfig *tls.Config, address, code string) error {
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(sendCtx, "tcp", net.JoinHostPort(cfg.host, cfg.port))
	if err != nil {
		return errors.New("邮件服务连接失败")
	}
	defer conn.Close()
	stop := context.AfterFunc(sendCtx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := sendCtx.Deadline()
	_ = conn.SetDeadline(deadline)
	var smtpConn net.Conn = conn
	if cfg.implicit {
		smtpConn = tls.Client(conn, tlsConfig)
	}
	client, err := smtp.NewClient(smtpConn, cfg.host)
	if err != nil {
		return errors.New("邮件服务握手失败")
	}
	defer client.Close()
	if !cfg.implicit {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("邮件服务不支持 TLS")
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return errors.New("邮件服务 TLS 验证失败")
		}
	}
	if err = client.Auth(smtp.PlainAuth("", cfg.user, cfg.password, cfg.host)); err != nil {
		return errors.New("邮件服务认证失败")
	}
	if err = client.Mail(cfg.from); err != nil {
		return errors.New("邮件服务发件失败")
	}
	if err = client.Rcpt(address); err != nil {
		return errors.New("邮件服务收件失败")
	}
	body, err := client.Data()
	if err != nil {
		return errors.New("邮件服务提交失败")
	}
	_, err = fmt.Fprintf(body, "From: %s\r\nTo: %s\r\nSubject: MLC verification code\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nYour MLC verification code is %s. It expires in 5 minutes. If you did not request this code, ignore this email.\r\n", cfg.from, address, code)
	if err != nil || body.Close() != nil {
		return errors.New("邮件发送失败")
	}
	// DATA 的 250 已确认接收；QUIT 丢包不能导致已投递的验证码失效。
	return nil
}

package hgemail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
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

// HGSMTPSender 从进程环境读取 SMTP 配置；没有配置时拒绝发送，不在代码中存储凭据。
type HGSMTPSender struct{}

// NewHGSMTPSender 提供只在实际发送时读取配置的 SMTP 适配器。
func NewHGSMTPSender() *HGSMTPSender { return &HGSMTPSender{} }

// SendCode 通过校验证书的 STARTTLS 连接发送邮件；连接和整个 SMTP 会话均有时限。
func (s *HGSMTPSender) SendCode(ctx context.Context, address, code string) error {
	host := strings.TrimSpace(os.Getenv("MLC_SMTP_HOST"))
	port := strings.TrimSpace(os.Getenv("MLC_SMTP_PORT"))
	from := strings.TrimSpace(os.Getenv("MLC_SMTP_FROM"))
	user := os.Getenv("MLC_SMTP_USER")
	password := os.Getenv("MLC_SMTP_PASSWORD")
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Address != address || strings.ContainsAny(address, "\r\n") || len(address) > 255 || len(code) != 6 {
		return errors.New("邮箱验证码参数无效")
	}
	if host == "" || from == "" || user == "" || password == "" {
		return errors.New("邮件服务未配置")
	}
	fromAddress, err := mail.ParseAddress(from)
	if err != nil || fromAddress.Address != from || strings.ContainsAny(from, "\r\n") {
		return errors.New("邮件发件人配置无效")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return errors.New("邮件服务端口无效")
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(sendCtx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return errors.New("邮件服务连接失败")
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return errors.New("邮件服务握手失败")
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return errors.New("邮件服务不支持 TLS")
	}
	if err = client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
		return errors.New("邮件服务 TLS 验证失败")
	}
	if err = client.Auth(smtp.PlainAuth("", user, password, host)); err != nil {
		return errors.New("邮件服务认证失败")
	}
	if err = client.Mail(fromAddress.Address); err != nil {
		return errors.New("邮件服务发件失败")
	}
	if err = client.Rcpt(parsed.Address); err != nil {
		return errors.New("邮件服务收件失败")
	}
	body, err := client.Data()
	if err != nil {
		return errors.New("邮件服务提交失败")
	}
	_, err = fmt.Fprintf(body, "From: %s\r\nTo: %s\r\nSubject: MLC verification code\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nYour MLC verification code is %s. It expires in 5 minutes.\r\n", from, address, code)
	if err != nil || body.Close() != nil {
		return errors.New("邮件发送失败")
	}
	return client.Quit()
}

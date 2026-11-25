package hgemail

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHGEmailAddresses(t *testing.T) {
	for _, address := range []string{"mail.test+tag@gmail.com", "name@icloud.com", "123@example.QQ.COM", "demo@qq.com"} {
		if _, err := HGNormalizeAddress(address); err != nil {
			t.Fatal("常用邮箱格式被拒绝")
		}
	}
	for _, address := range []string{"name@gmail.com\r\nBcc: other@example.org", "name..a@icloud.com", "a@qq..com", "not-an-email", "a@-bad.example"} {
		if _, err := HGNormalizeAddress(address); err == nil {
			t.Fatal("无效地址不应发送")
		}
	}
}

// hgSMTPFixture 在回环地址模拟邮件提交；不使用真实 SMTP 凭据或外部邮箱。
func hgSMTPFixture(t *testing.T, implicit, startTLS bool) (hgSMTPConfig, *tls.Config, <-chan error) {
	t.Helper()
	certServer := httptest.NewTLSServer(nil)
	cert := certServer.TLS.Certificates[0]
	roots := x509.NewCertPool()
	roots.AddCert(certServer.Certificate())
	certServer.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	cfg := hgSMTPConfig{
		host: "127.0.0.1", port: fmt.Sprint(listener.Addr().(*net.TCPAddr).Port),
		from: "sender@example.net", user: "fixture", password: "local-test-only", implicit: implicit,
	}
	serverTLS := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	done := make(chan error, 1)
	go func() {
		done <- func() error {
			raw, err := listener.Accept()
			if err != nil {
				return err
			}
			defer raw.Close()
			_ = raw.SetDeadline(time.Now().Add(3 * time.Second))
			conn := raw
			if implicit {
				conn = tls.Server(raw, serverTLS)
			}
			reader := bufio.NewReader(conn)
			say := func(line string) error { _, err := fmt.Fprint(conn, line); return err }
			if err := say("220 fixture SMTP\r\n"); err != nil {
				return err
			}
			secured := implicit
			for {
				command, err := reader.ReadString('\n')
				if err != nil {
					if !startTLS && !secured {
						return nil
					}
					return err
				}
				parts := strings.Fields(command)
				if len(parts) == 0 {
					return fmt.Errorf("SMTP 请求格式无效")
				}
				switch parts[0] {
				case "EHLO":
					switch {
					case secured:
						err = say("250-fixture\r\n250 AUTH PLAIN\r\n")
					case startTLS:
						err = say("250-fixture\r\n250 STARTTLS\r\n")
					default:
						err = say("250 fixture\r\n")
					}
				case "STARTTLS":
					if err := say("220 ready to start TLS\r\n"); err != nil {
						return err
					}
					conn = tls.Server(raw, serverTLS)
					reader = bufio.NewReader(conn)
					secured = true
				case "AUTH":
					if !secured {
						return fmt.Errorf("禁止明文发送认证")
					}
					err = say("235 authenticated\r\n")
				case "MAIL", "RCPT":
					err = say("250 ok\r\n")
				case "DATA":
					if err := say("354 send data\r\n"); err != nil {
						return err
					}
					var content strings.Builder
					for {
						line, err := reader.ReadString('\n')
						if err != nil {
							return err
						}
						if line == ".\r\n" {
							break
						}
						content.WriteString(line)
					}
					if !strings.Contains(content.String(), "To: receiver@example.org\r\n") ||
						!strings.Contains(content.String(), "verification code is 012345") {
						return fmt.Errorf("邮件正文或收件地址不正确")
					}
					return say("250 queued\r\n")
				default:
					return fmt.Errorf("SMTP 步骤不符合预期")
				}
				if err != nil {
					return err
				}
			}
		}()
	}()
	return cfg, &tls.Config{RootCAs: roots, ServerName: cfg.host, MinVersion: tls.VersionTLS12}, done
}

func TestHGSMTPCodeRequiresVerifiedTLS(t *testing.T) {
	for _, tc := range []struct {
		name               string
		implicit, startTLS bool
		ok                 bool
	}{
		{"STARTTLS", false, true, true},
		{"implicit TLS", true, false, true},
		{"reject plaintext", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, tlsConfig, done := hgSMTPFixture(t, tc.implicit, tc.startTLS)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := hgSendSMTP(ctx, cfg, tlsConfig, "receiver@example.org", "012345")
			if (err == nil) != tc.ok {
				t.Fatalf("SMTP 结果不符: %v", err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("reject invalid certificate", func(t *testing.T) {
		cfg, tlsConfig, done := hgSMTPFixture(t, true, false)
		tlsConfig.ServerName = "wrong.example.org"
		if err := hgSendSMTP(context.Background(), cfg, tlsConfig, "receiver@example.org", "012345"); err == nil {
			t.Fatal("证书主机名不匹配时不应发送")
		}
		<-done
	})
}

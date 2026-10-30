// Package transport 封装 IM 外部协议，不承载成员授权或业务状态。
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HGEndpoint 仅接受配置端点；不接受用户传入的 URL，不跟随重定向。
func HGEndpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("IM 服务端点不合法")
	}
	if u.Scheme == "http" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1" {
		return nil, fmt.Errorf("IM 非本机端点必须使用 HTTPS")
	}
	return u, nil
}

func hgClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxConnsPerHost, t.MaxIdleConnsPerHost = 16, 8
	return &http.Client{Transport: t, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

type hgStatusError struct {
	status int /* 外部 HTTP 状态，不包含响应正文。 */
}

func (e hgStatusError) Error() string {
	return fmt.Sprintf("IM 外部服务状态异常: %d", e.status)
}

func hgPost(ctx context.Context, client *http.Client, endpoint, auth string, request, response any) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建 IM 外部请求失败")
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.HasPrefix(auth, "apikey ") {
		req.Header.Set("X-API-Key", strings.TrimPrefix(auth, "apikey "))
	} else {
		req.Header.Set("Authorization", auth)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("IM 外部服务请求失败")
	} // 不回显包含签名、对象 Key 的 URL。
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return hgStatusError{status: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(data) > 65536 {
		return fmt.Errorf("IM 外部响应超限或读取失败")
	}
	if response == nil {
		return nil
	}
	if err := json.Unmarshal(data, response); err != nil {
		return fmt.Errorf("IM 外部响应格式错误")
	}
	return nil
}

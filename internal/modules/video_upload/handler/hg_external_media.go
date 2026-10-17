package VideoUploadHandlerPackage

import (
	hgContext "MLC_GO/internal/pkg/hg_context"
	hgResponse "MLC_GO/internal/response"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// hgMediaSession 是debug单实例播放凭证，媒体地址仅保存在服务端，30分钟过期。
type hgMediaSession struct {
	hgID      string    // 绑定分P身份。
	hgURL     string    // 上游临时地址，不输出到日志。
	hgExpires time.Time // 凭证到期时间。
}

// hgMediaProxy 限制会话数和活动请求数，不缓存媒体内容。
type hgMediaProxy struct {
	hgMu       sync.Mutex                // 保护有界会话表。
	hgSessions map[string]hgMediaSession // 最多256个会话。
	hgSlots    chan struct{}             // 最多8个活动上游请求。
	hgClient   *http.Client              // 禁止重定向及私网拨号。
}

func hgNewMediaProxy() *hgMediaProxy {
	transport := &http.Transport{TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, MaxIdleConns: 16, MaxIdleConnsPerHost: 4, IdleConnTimeout: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !ip.IP.IsGlobalUnicast() || ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() {
				return nil, errors.New("禁止私网媒体地址")
			}
		}
		if len(ips) == 0 {
			return nil, errors.New("媒体域名解析失败")
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
	return &hgMediaProxy{hgSessions: make(map[string]hgMediaSession), hgSlots: make(chan struct{}, 8), hgClient: &http.Client{Transport: transport, Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("禁止媒体重定向") }}}
}

// hgMediaCookieName 按CID隔离不同标签页凭证，凭证不进入URL或访问日志。
func hgMediaCookieName(id string) string {
	sum := sha256.Sum256([]byte(id))
	return "hg_media_" + hex.EncodeToString(sum[:8])
}

func hgAllowedMediaURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && strings.HasSuffix(u.Hostname(), ".bilivideo.com") && strings.HasSuffix(u.Path, ".mp4")
}

// HGExternalPlayback 经过既有签名/JWT后签发HttpOnly媒体凭证，仅debug启用。
func (h *Handler) HGExternalPlayback(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("SERVER_ENV") != "debug" {
		http.Error(w, "媒体代理仅debug启用", http.StatusServiceUnavailable)
		return
	}
	if _, ok := hgContext.CurrentUserID(r); !ok {
		http.Error(w, "未登录", 401)
		return
	}
	select {
	case h.hgMedia.hgSlots <- struct{}{}:
		defer func() { <-h.hgMedia.hgSlots }()
	default:
		http.Error(w, "媒体服务繁忙", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id := r.URL.Query().Get("videoId")
	bvid, cid, err := h.service.HGValidateExternalPart(ctx, id)
	if err != nil {
		http.Error(w, "分P不可播放", 404)
		return
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.bilibili.com/x/player/playurl?"+url.Values{"bvid": {bvid}, "cid": {cid}, "qn": {"16"}, "fnval": {"1"}, "fnver": {"0"}, "fourk": {"0"}}.Encode(), nil)
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := h.hgMedia.hgClient.Do(req)
	if err != nil {
		http.Error(w, "播放源暂不可用", 502)
		return
	}
	defer resp.Body.Close()
	// 上游播放协议：只接受单段MP4；DASH/多段不静默截断。
	var data struct {
		Code int `json:"code"`
		Data struct {
			DURL []struct {
				URL    string   `json:"url"`
				Backup []string `json:"backup_url"`
			} `json:"durl"`
		} `json:"data"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data) != nil || data.Code != 0 || len(data.Data.DURL) != 1 {
		http.Error(w, "当前视频没有可用单段MP4", 502)
		return
	}
	source := ""
	for _, candidate := range append(data.Data.DURL[0].Backup, data.Data.DURL[0].URL) {
		if hgAllowedMediaURL(candidate) {
			source = candidate
			break
		}
	}
	if source == "" {
		http.Error(w, "媒体源不在允许范围", 502)
		return
	}
	random := make([]byte, 32)
	if _, err = rand.Read(random); err != nil {
		http.Error(w, "凭证生成失败", 500)
		return
	}
	token := hex.EncodeToString(random)
	expires := time.Now().Add(30 * time.Minute)
	h.hgMedia.hgMu.Lock()
	for key, session := range h.hgMedia.hgSessions {
		if time.Now().After(session.hgExpires) {
			delete(h.hgMedia.hgSessions, key)
		}
	}
	if len(h.hgMedia.hgSessions) >= 256 {
		h.hgMedia.hgMu.Unlock()
		http.Error(w, "播放会话已满", 503)
		return
	}
	h.hgMedia.hgSessions[token] = hgMediaSession{id, source, expires}
	h.hgMedia.hgMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: hgMediaCookieName(id), Value: token, Path: "/api/v1/video_upload/external-stream", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: 1800})
	w.Header().Set("Cache-Control", "no-store")
	hgResponse.SuccessResult(w, r, map[string]any{"url": "/api/v1/video_upload/external-stream?videoId=" + url.QueryEscape(id), "expiresAt": expires.Unix(), "quality": "360P"})
}

var hgMediaRange = regexp.MustCompile(`^bytes=[0-9]{1,15}-[0-9]{0,15}$`)

// HGExternalStream 以HttpOnly能力凭证鉴权，原生video无法携带API签名头。每次最多8MiB，浏览器按Range续取。
func (h *Handler) HGExternalStream(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("SERVER_ENV") != "debug" {
		http.Error(w, "媒体代理仅debug启用", 503)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "方法不支持", 405)
		return
	}
	id := r.URL.Query().Get("videoId")
	cookie, err := r.Cookie(hgMediaCookieName(id))
	if err != nil {
		http.Error(w, "播放凭证缺失", 401)
		return
	}
	h.hgMedia.hgMu.Lock()
	session, ok := h.hgMedia.hgSessions[cookie.Value]
	h.hgMedia.hgMu.Unlock()
	if !ok || session.hgID != id || time.Now().After(session.hgExpires) {
		http.Error(w, "播放凭证已过期，请刷新播放源", 401)
		return
	}
	select {
	case h.hgMedia.hgSlots <- struct{}{}:
		defer func() { <-h.hgMedia.hgSlots }()
	default:
		w.Header().Set("Retry-After", "2")
		http.Error(w, "媒体服务繁忙", 503)
		return
	}
	start, end := int64(0), int64((8<<20)-1)
	if value := r.Header.Get("Range"); value != "" {
		if !hgMediaRange.MatchString(value) {
			http.Error(w, "仅支持单段Range", 416)
			return
		}
		var requested int64
		n, _ := fmt.Sscanf(value, "bytes=%d-%d", &start, &requested)
		if n == 2 {
			if requested < start {
				http.Error(w, "Range无效", 416)
				return
			}
			if requested < start+(8<<20)-1 {
				end = requested
			} else {
				end = start + (8 << 20) - 1
			}
		} else {
			end = start + (8 << 20) - 1
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, r.Method, session.hgURL, nil)
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := h.hgMedia.hgClient.Do(req)
	if err != nil {
		http.Error(w, "媒体读取失败", 502)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 416 {
		w.Header().Set("Content-Range", resp.Header.Get("Content-Range"))
		w.WriteHeader(416)
		return
	}
	if resp.StatusCode != 206 || resp.ContentLength < 0 || resp.ContentLength > 8<<20 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "video/mp4") {
		http.Error(w, "媒体源响应不可用，请刷新播放源", 502)
		return
	}
	var hgStart, hgEnd, hgTotal int64
	if hgN, _ := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%d", &hgStart, &hgEnd, &hgTotal); hgN != 3 || hgStart != start || hgEnd < hgStart || hgEnd > end || hgTotal <= hgEnd || hgEnd-hgStart+1 != resp.ContentLength {
		http.Error(w, "媒体Range响应不一致", http.StatusBadGateway)
		return
	}
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		w.Header().Set(key, resp.Header.Get(key))
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(206)
	if r.Method == http.MethodGet {
		if _, err = io.CopyN(w, resp.Body, resp.ContentLength); err != nil {
			panic(http.ErrAbortHandler)
		}
	}
}

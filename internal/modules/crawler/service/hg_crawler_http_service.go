package service

import (
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const hgCrawlerMaxResponseBytes = 2 << 20

var (
	// ErrHGCrawlerInvalidRequest indicates malformed or unsupported outbound request input.
	ErrHGCrawlerInvalidRequest = errors.New("crawler request is invalid")
	// ErrHGCrawlerUnsafeTarget indicates a target outside the exact host and public-network policy.
	ErrHGCrawlerUnsafeTarget = errors.New("crawler target is not allowed")
	// ErrHGCrawlerRedirect indicates that the upstream attempted a redirect, which is never followed.
	ErrHGCrawlerRedirect = errors.New("crawler redirect is not allowed")
)

// HGTargetPolicy is an immutable exact-host outbound policy. HTTPS is required unless AllowHTTP is true.
type HGTargetPolicy struct {
	AllowedHosts map[string]struct{}
	AllowHTTP    bool
}

// NewHGTargetPolicy normalizes a case-insensitive exact-host allowlist without permitting subdomain suffix matches.
func NewHGTargetPolicy(allowedHosts []string, allowHTTP bool) (HGTargetPolicy, error) {
	policy := HGTargetPolicy{AllowedHosts: make(map[string]struct{}, len(allowedHosts)), AllowHTTP: allowHTTP}
	for _, value := range allowedHosts {
		host := hgNormalizeCrawlerHost(value)
		_, hostIsIP := netip.ParseAddr(host)
		if host == "" || strings.ContainsAny(host, "/@") || (strings.Contains(host, ":") && hostIsIP != nil) {
			return HGTargetPolicy{}, fmt.Errorf("%w: invalid allowed host", ErrHGCrawlerInvalidRequest)
		}
		policy.AllowedHosts[host] = struct{}{}
	}
	if len(policy.AllowedHosts) == 0 {
		return HGTargetPolicy{}, fmt.Errorf("%w: allowed hosts are required", ErrHGCrawlerInvalidRequest)
	}
	return policy, nil
}

// ValidateTarget parses a URL and applies scheme, credentials, fragment, exact-host, and literal-IP checks.
func (p HGTargetPolicy) ValidateTarget(rawURL string) (*url.URL, error) {
	if len(rawURL) == 0 || len(rawURL) > 4096 {
		return nil, ErrHGCrawlerInvalidRequest
	}
	target, err := url.Parse(rawURL)
	if err != nil || target.Hostname() == "" || target.User != nil || target.Fragment != "" {
		return nil, ErrHGCrawlerInvalidRequest
	}
	scheme := strings.ToLower(target.Scheme)
	if scheme != "https" && !(p.AllowHTTP && scheme == "http") {
		return nil, ErrHGCrawlerUnsafeTarget
	}
	host := hgNormalizeCrawlerHost(target.Hostname())
	if _, allowed := p.AllowedHosts[host]; !allowed {
		return nil, ErrHGCrawlerUnsafeTarget
	}
	if ip, parseErr := netip.ParseAddr(host); parseErr == nil && !hgCrawlerPublicIP(ip.Unmap()) {
		return nil, ErrHGCrawlerUnsafeTarget
	}
	if port := target.Port(); port != "" && !((scheme == "https" && port == "443") || (scheme == "http" && port == "80")) {
		return nil, ErrHGCrawlerUnsafeTarget
	}
	return target, nil
}

// HGHTTPResult contains bounded raw response data used by formal crawler execution.
type HGHTTPResult struct {
	URL        string
	StatusCode int
	Status     string
	Header     http.Header
	Body       []byte
	Cost       time.Duration
}

// HGSafeHTTPService performs bounded outbound requests with proxying and redirects disabled.
type HGSafeHTTPService struct {
	policy    HGTargetPolicy
	client    *http.Client
	userAgent string
}

// NewHGSafeHTTPService builds an SSRF-resistant client that verifies every resolved IP before dialing.
func NewHGSafeHTTPService(policy HGTargetPolicy, userAgent string) (*HGSafeHTTPService, error) {
	if len(policy.AllowedHosts) == 0 {
		return nil, errors.New("crawler target policy is required")
	}
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" || strings.ContainsAny(userAgent, "\r\n") {
		return nil, errors.New("crawler user agent is required")
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.MaxIdleConnsPerHost = 4
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("split crawler target address: %w", err)
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve crawler target: %w", err)
		}
		if len(addresses) == 0 {
			return nil, errors.New("crawler target has no resolved address")
		}
		for _, addressIP := range addresses {
			if !hgCrawlerPublicIP(addressIP.Unmap()) {
				return nil, ErrHGCrawlerUnsafeTarget
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
	}
	return &HGSafeHTTPService{
		policy: policy,
		client: &http.Client{
			Transport:     transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return ErrHGCrawlerRedirect },
		},
		userAgent: userAgent,
	}, nil
}

// Execute validates and performs one GET or POST request and returns at most 2 MiB of raw response bytes.
func (s *HGSafeHTTPService) Execute(ctx context.Context, req CrawlerDtoPackage.HGDebugRequest) (HGHTTPResult, error) {
	if s == nil || s.client == nil {
		return HGHTTPResult{}, errors.New("crawler HTTP service is not initialized")
	}
	target, method, timeout, err := s.normalizeRequest(req)
	if err != nil {
		return HGHTTPResult{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}
	httpRequest, err := http.NewRequestWithContext(requestCtx, method, target.String(), body)
	if err != nil {
		return HGHTTPResult{}, ErrHGCrawlerInvalidRequest
	}
	httpRequest.Header.Set("User-Agent", s.userAgent)
	httpRequest.Header.Set("Accept", "application/json")
	for key, value := range req.Headers {
		if err := hgSetCrawlerHeader(httpRequest.Header, key, value); err != nil {
			return HGHTTPResult{}, err
		}
	}
	startedAt := time.Now()
	response, err := s.client.Do(httpRequest)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return HGHTTPResult{}, context.DeadlineExceeded
		}
		if errors.Is(err, ErrHGCrawlerRedirect) {
			return HGHTTPResult{}, ErrHGCrawlerRedirect
		}
		return HGHTTPResult{}, fmt.Errorf("request crawler target: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, hgCrawlerMaxResponseBytes+1))
	if err != nil {
		return HGHTTPResult{}, fmt.Errorf("read crawler response: %w", err)
	}
	if len(data) > hgCrawlerMaxResponseBytes {
		return HGHTTPResult{}, errors.New("crawler response exceeds 2 MiB limit")
	}
	return HGHTTPResult{URL: target.String(), StatusCode: response.StatusCode, Status: response.Status, Header: response.Header.Clone(), Body: data, Cost: time.Since(startedAt)}, nil
}

// ValidateRequest applies the complete static request policy without performing DNS or network I/O.
func (s *HGSafeHTTPService) ValidateRequest(req CrawlerDtoPackage.HGDebugRequest) error {
	if s == nil {
		return errors.New("crawler HTTP service is not initialized")
	}
	_, _, _, err := s.normalizeRequest(req)
	return err
}

func (s *HGSafeHTTPService) normalizeRequest(req CrawlerDtoPackage.HGDebugRequest) (*url.URL, string, time.Duration, error) {
	if len(req.Headers) > 16 || len(req.Params) > 32 || len(req.Body) > 64<<10 {
		return nil, "", 0, ErrHGCrawlerInvalidRequest
	}
	normalizedURL, err := hgNormalizeBilibiliVideoURL(req.URL)
	if err != nil {
		return nil, "", 0, err
	}
	target, err := s.policy.ValidateTarget(normalizedURL)
	if err != nil {
		return nil, "", 0, err
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodPost {
		return nil, "", 0, ErrHGCrawlerInvalidRequest
	}
	query := target.Query()
	for key, value := range req.Params {
		if len(key) == 0 || len(key) > 128 || len(value) > 2048 {
			return nil, "", 0, ErrHGCrawlerInvalidRequest
		}
		query.Set(key, value)
	}
	target.RawQuery = query.Encode()
	timeoutMS := req.TimeoutMS
	if timeoutMS == 0 {
		timeoutMS = 10000
	}
	if timeoutMS < 500 || timeoutMS > 10000 {
		return nil, "", 0, ErrHGCrawlerInvalidRequest
	}
	return target, method, time.Duration(timeoutMS) * time.Millisecond, nil
}

// hgNormalizeBilibiliVideoURL converts a public BV webpage into the allowlisted detail API endpoint.
func hgNormalizeBilibiliVideoURL(rawURL string) (string, error) {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL, nil
	}
	if !strings.EqualFold(target.Hostname(), "www.bilibili.com") {
		return rawURL, nil
	}
	if target.Scheme != "https" || target.User != nil {
		return "", ErrHGCrawlerInvalidRequest
	}
	parts := strings.Split(strings.Trim(target.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "video" || len(parts[1]) < 5 || len(parts[1]) > 32 || !strings.HasPrefix(strings.ToUpper(parts[1]), "BV") {
		return "", ErrHGCrawlerInvalidRequest
	}
	for _, character := range parts[1] {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')) {
			return "", ErrHGCrawlerInvalidRequest
		}
	}
	endpoint := &url.URL{Scheme: "https", Host: "api.bilibili.com", Path: "/x/web-interface/view"}
	query := endpoint.Query()
	query.Set("bvid", parts[1])
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func hgSetCrawlerHeader(headers http.Header, key, value string) error {
	key = http.CanonicalHeaderKey(strings.TrimSpace(key))
	if key == "" || len(key) > 64 || len(value) > 1024 || strings.ContainsAny(value, "\r\n") {
		return ErrHGCrawlerInvalidRequest
	}
	allowed := map[string]bool{"Accept": true, "Accept-Language": true, "Content-Type": true, "Referer": true, "User-Agent": true}
	if !allowed[key] {
		return fmt.Errorf("%w: header %s is not allowed", ErrHGCrawlerInvalidRequest, key)
	}
	headers.Set(key, value)
	return nil
}

func hgNormalizeCrawlerHost(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

func hgCrawlerPublicIP(ip netip.Addr) bool {
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	for _, prefix := range []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("2001:db8::/32"),
	} {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

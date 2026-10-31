package transport

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hgObjectKey = regexp.MustCompile(`^(quarantine|sealed)/[a-f0-9]{32}$`)
var hgByteRange = regexp.MustCompile(`^bytes=([0-9]{1,12})-([0-9]{0,12})$`)
var hgContentRange = regexp.MustCompile(`^bytes ([0-9]{1,12})-([0-9]{1,12})/([0-9]{1,12})$`)

const (
	HGMaxImageBytes = 10 << 20  // 图片上限，字节。
	HGMaxMediaBytes = 128 << 20 // 音频、普通文件上限，字节。
	HGMaxVideoBytes = 300 << 20 // 视频上限 300MiB，包含边界值。
)

// HGMediaSizeAllowed 使用 MIME 同步审核与 S3 请求边界，禁止只扩大预签名一侧。
func HGMediaSizeAllowed(mime string, size int64) bool {
	if size < 1 {
		return false
	}
	switch {
	case strings.HasPrefix(mime, "image/"):
		return size <= HGMaxImageBytes
	case strings.HasPrefix(mime, "video/"):
		return size <= HGMaxVideoBytes
	default:
		return size <= HGMaxMediaBytes
	}
}

// HGSeaweedFS 仅通过 S3 SigV4 访问私有桶；上传身份只能写 quarantine，审核器独占 sealed 写权限。
type HGSeaweedFS struct {
	endpoint string       // 客户端可达 S3 根端点，不包含 bucket 或查询参数。
	region   string       // SigV4 region，须与 SeaweedFS 一致。
	bucket   string       // 私有桶，禁止匿名访问。
	key      string       // S3 Access Key，仅后端持有。
	secret   string       // S3 Secret Key，不记录日志。
	client   *http.Client // GET/HEAD 连接池，禁止重定向。
}

func HGNewSeaweedFS(endpoint, region, bucket, key, secret string) (*HGSeaweedFS, error) {
	u, err := HGEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	if (u.Path != "" && u.Path != "/") || !regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`).MatchString(bucket) || region == "" || key == "" || secret == "" {
		return nil, fmt.Errorf("SeaweedFS S3 配置不完整")
	}
	client := hgClient()
	client.Timeout = 270 * time.Second
	return &HGSeaweedFS{endpoint: strings.TrimRight(endpoint, "/"), region: region, bucket: bucket, key: key, secret: secret, client: client}, nil
}

func hgMAC(key []byte, value string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(value))
	return h.Sum(nil)
}
func hgHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// hgSign 实现 AWS SigV4 query 签名，路径限定为服务端随机对象键，上传必须匹配类型与字节数。
func (s *HGSeaweedFS) hgSign(method, key, mime string, size int64, ttl time.Duration) (string, error) {
	return s.hgSignObject(method, key, mime, size, ttl, false)
}

// hgSignObject 在封存写入时额外签名 If-None-Match；部署前须实测存储是否真正执行条件写入。
func (s *HGSeaweedFS) hgSignObject(method, key, mime string, size int64, ttl time.Duration, createOnly bool) (string, error) {
	if !hgObjectKey.MatchString(key) || ttl <= 0 || ttl > 10*time.Minute {
		return "", fmt.Errorf("私有对象签名参数错误")
	}
	if createOnly && (method != http.MethodPut || !strings.HasPrefix(key, "sealed/")) {
		return "", fmt.Errorf("条件写入只允许封存对象")
	}
	u, _ := url.Parse(s.endpoint + "/" + s.bucket + "/" + key)
	now := time.Now().UTC()
	date := now.Format("20060102")
	stamp := now.Format("20060102T150405Z")
	scope := date + "/" + s.region + "/s3/aws4_request"
	headers := "host"
	canonical := "host:" + u.Host + "\n"
	if method == http.MethodPut {
		headers = "content-length;content-type;host;x-amz-acl"
		canonical = "content-length:" + strconv.FormatInt(size, 10) + "\ncontent-type:" + mime + "\nhost:" + u.Host + "\nx-amz-acl:private\n"
		if createOnly {
			headers = "content-length;content-type;host;if-none-match;x-amz-acl"
			canonical = "content-length:" + strconv.FormatInt(size, 10) + "\ncontent-type:" + mime + "\nhost:" + u.Host + "\nif-none-match:*\nx-amz-acl:private\n"
		}
	}
	q := url.Values{"X-Amz-Algorithm": {"AWS4-HMAC-SHA256"}, "X-Amz-Credential": {s.key + "/" + scope}, "X-Amz-Date": {stamp}, "X-Amz-Expires": {strconv.FormatInt(int64(ttl/time.Second), 10)}, "X-Amz-SignedHeaders": {headers}}
	query := strings.ReplaceAll(q.Encode(), "+", "%20")
	request := method + "\n" + u.EscapedPath() + "\n" + query + "\n" + canonical + "\n" + headers + "\nUNSIGNED-PAYLOAD"
	keyBytes := hgMAC([]byte("AWS4"+s.secret), date)
	keyBytes = hgMAC(keyBytes, s.region)
	keyBytes = hgMAC(keyBytes, "s3")
	keyBytes = hgMAC(keyBytes, "aws4_request")
	q.Set("X-Amz-Signature", hex.EncodeToString(hgMAC(keyBytes, "AWS4-HMAC-SHA256\n"+stamp+"\n"+scope+"\n"+hgHash(request))))
	u.RawQuery = strings.ReplaceAll(q.Encode(), "+", "%20")
	return u.String(), nil
}

func (s *HGSeaweedFS) HGUploadURL(key, mime string, size int64) (string, error) {
	if !strings.HasPrefix(key, "quarantine/") || !HGMediaSizeAllowed(mime, size) {
		return "", fmt.Errorf("上传参数错误")
	}
	return s.hgSign(http.MethodPut, key, mime, size, 10*time.Minute)
}

// HGReviewOpenQuarantine 使用审核器专属只读身份获取待审字节，不向浏览器下发下载签名。
func (s *HGSeaweedFS) HGReviewOpenQuarantine(ctx context.Context, key string, size int64) (io.ReadCloser, error) {
	if !strings.HasPrefix(key, "quarantine/") || size < 1 || size > HGMaxVideoBytes {
		return nil, fmt.Errorf("待审对象参数错误")
	}
	link, err := s.hgSign(http.MethodGet, key, "", 0, time.Minute)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, fmt.Errorf("创建待审请求失败")
	}
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("读取待审对象失败")
	}
	if resp.StatusCode != http.StatusOK || resp.ContentLength != size || resp.Header.Get("Content-Encoding") != "" && resp.Header.Get("Content-Encoding") != "identity" {
		resp.Body.Close()
		return nil, fmt.Errorf("待审对象响应不合法")
	}
	return resp.Body, nil
}

// HGReviewSeal 仅允许首次创建 sealed 对象，412 表示已有对象且须由调用方重新读回核验。
func (s *HGSeaweedFS) HGReviewSeal(ctx context.Context, key, mime string, size int64, body io.Reader) (bool, error) {
	if !strings.HasPrefix(key, "sealed/") || !HGMediaSizeAllowed(mime, size) {
		return false, fmt.Errorf("封存参数错误")
	}
	link, err := s.hgSignObject(http.MethodPut, key, mime, size, time.Minute, true)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, link, body)
	if err != nil {
		return false, fmt.Errorf("创建封存请求失败")
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", mime)
	req.Header.Set("If-None-Match", "*")
	req.Header.Set("x-amz-acl", "private")
	resp, err := s.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("封存请求失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusPreconditionFailed {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return false, fmt.Errorf("封存对象未确认")
	}
	return true, nil
}

// HGOpen 私有下载仅供已授权 Go handler 代理，签名 URL 不下发；单件最多 300MiB。
func (s *HGSeaweedFS) HGOpen(ctx context.Context, key, byteRange string) (*http.Response, error) {
	if !strings.HasPrefix(key, "sealed/") {
		return nil, fmt.Errorf("对象尚未审核封存")
	}
	var hgStart int64
	hgEnd := int64(-1)
	if byteRange != "" {
		parts := hgByteRange.FindStringSubmatch(byteRange)
		if len(parts) != 3 {
			return nil, fmt.Errorf("只支持单段字节范围")
		}
		hgStart, _ = strconv.ParseInt(parts[1], 10, 64)
		if parts[2] != "" {
			hgEnd, _ = strconv.ParseInt(parts[2], 10, 64)
			if hgEnd < hgStart {
				return nil, fmt.Errorf("字节范围结束位置不得小于起点")
			}
		}
	}
	link, err := s.hgSign(http.MethodGet, key, "", 0, time.Minute)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, fmt.Errorf("创建私有读取请求失败")
	}
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	// 字节范围必须对应封存原始内容，禁止传输层自动解压改变大小和偏移。
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("读取私有对象失败")
	}
	if (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent) || resp.ContentLength < 1 || resp.ContentLength > HGMaxVideoBytes || (resp.Header.Get("Content-Encoding") != "" && resp.Header.Get("Content-Encoding") != "identity") {
		resp.Body.Close()
		return nil, fmt.Errorf("私有对象响应不合法")
	}
	if resp.StatusCode == http.StatusPartialContent {
		hgParts := hgContentRange.FindStringSubmatch(resp.Header.Get("Content-Range"))
		if byteRange == "" || len(hgParts) != 4 {
			resp.Body.Close()
			return nil, fmt.Errorf("私有对象缺少有效范围响应")
		}
		hgActualStart, _ := strconv.ParseInt(hgParts[1], 10, 64)
		hgActualEnd, _ := strconv.ParseInt(hgParts[2], 10, 64)
		hgTotal, _ := strconv.ParseInt(hgParts[3], 10, 64)
		hgExpectedEnd := hgTotal - 1
		if hgEnd >= 0 {
			hgExpectedEnd = min(hgEnd, hgExpectedEnd)
		}
		if hgTotal < 1 || hgTotal > HGMaxVideoBytes || hgActualStart != hgStart || hgActualEnd != hgExpectedEnd || hgActualEnd < hgActualStart || resp.ContentLength != hgActualEnd-hgActualStart+1 {
			resp.Body.Close()
			return nil, fmt.Errorf("私有对象范围与请求或长度不一致")
		}
		// 规范化已验证的总大小，handler 再与审核元数据核对。
		resp.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", hgActualStart, hgActualEnd, hgTotal))
	} else if resp.Header.Get("Content-Range") != "" {
		resp.Body.Close()
		return nil, fmt.Errorf("私有对象完整响应不应包含字节范围")
	}
	return resp, nil
}

func (s *HGSeaweedFS) HGClose() {
	if s != nil {
		s.client.CloseIdleConnections()
	}
}

// HGReviewResult 是受信审核器的完成契约；审核器必须真实计算摘要、探测格式、扫描并封存，不能回显申请值。
type HGReviewResult struct {
	Status     string `json:"status"`     // pending/ready/rejected；只有 ready 可发送。
	SHA256     string `json:"sha256"`     // 封存文件实际 SHA256，64 位小写十六进制。
	Size       int64  `json:"size"`       // 实际字节数。
	MIME       string `json:"mime"`       // 探测的真实 MIME。
	SealedKey  string `json:"sealedKey"`  // 必须是请求中指定的不可变对象 Key。
	Width      int    `json:"width"`      // 图片/视频宽，像素。
	Height     int    `json:"height"`     // 图片/视频高，像素。
	DurationMS int64  `json:"durationMs"` // 视频/音频时长，毫秒。
}

// HGReviewer 连接独立内容审核/格式验证服务；缺配置时媒体模块启动失败，不自动通过审核。
type HGReviewer struct {
	endpoint string       // 受信审核 API，服务端配置，禁止用户覆盖。
	token    string       // 审核服务凭据，只通过 Authorization 发送。
	client   *http.Client // 限流后的有界客户端。
}

func HGNewReviewer(endpoint, token string) (*HGReviewer, error) {
	if _, err := HGEndpoint(endpoint); err != nil {
		return nil, err
	}
	if len(token) < 32 {
		return nil, fmt.Errorf("媒体审核凭据缺失")
	}
	client := hgClient()
	// 300MiB 审核包含 S3 读写、病毒扫描和封存复核，超时必须重试而非默认通过。
	client.Timeout = 275 * time.Second
	return &HGReviewer{endpoint: endpoint, token: token, client: client}, nil
}

// HGReview 重试使用相同 mediaId；审核服务须幂等封存且拒绝覆盖已封存对象。
func (s *HGReviewer) HGReview(ctx context.Context, id, bucket, key, sha, mime string, size int64) (HGReviewResult, error) {
	var result HGReviewResult
	err := hgPost(ctx, s.client, s.endpoint, "Bearer "+s.token, map[string]any{"mediaId": id, "bucket": bucket, "objectKey": key, "sealedKey": "sealed/" + id, "sha256": sha, "mime": mime, "size": size}, &result)
	return result, err
}
func (s *HGReviewer) HGClose() {
	if s != nil {
		s.client.CloseIdleConnections()
	}
}

package transport

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hgObjectKey = regexp.MustCompile(`^(quarantine|sealed)/[a-f0-9]{32}$`)
var hgByteRange = regexp.MustCompile(`^bytes=([0-9]{1,12})-([0-9]{0,12})$`)

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
	client.Timeout = 30 * time.Second
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
	if !hgObjectKey.MatchString(key) || ttl <= 0 || ttl > 10*time.Minute {
		return "", fmt.Errorf("私有对象签名参数错误")
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
	if !strings.HasPrefix(key, "quarantine/") || size < 1 || size > 128<<20 {
		return "", fmt.Errorf("上传参数错误")
	}
	return s.hgSign(http.MethodPut, key, mime, size, 10*time.Minute)
}

// HGOpen 私有下载仅供已授权 Go handler 代理，签名 URL 不下发；单次最多 128 MiB，超时 30 秒。
func (s *HGSeaweedFS) HGOpen(ctx context.Context, key, byteRange string) (*http.Response, error) {
	if !strings.HasPrefix(key, "sealed/") {
		return nil, fmt.Errorf("对象尚未审核封存")
	}
	if byteRange != "" {
		parts := hgByteRange.FindStringSubmatch(byteRange)
		if len(parts) != 3 { return nil, fmt.Errorf("只支持单段字节范围") }
		start, _ := strconv.ParseInt(parts[1], 10, 64)
		if parts[2]!="" {end,_:=strconv.ParseInt(parts[2],10,64);if end<start{return nil,fmt.Errorf("字节范围结束位置不得小于起点")}}
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
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("读取私有对象失败")
	}
	if (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent) || resp.ContentLength < 0 || resp.ContentLength > 128<<20 {
		resp.Body.Close()
		return nil, fmt.Errorf("私有对象响应不合法")
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
	return &HGReviewer{endpoint: endpoint, token: token, client: hgClient()}, nil
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

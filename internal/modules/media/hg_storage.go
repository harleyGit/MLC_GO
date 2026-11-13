package media

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var hgBucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// HGObjectStore 是媒体模块的 SeaweedFS S3 边界；上传只能进入隔离桶，不提供公开读取。
type HGObjectStore interface {
	HGSignUpload(ctx context.Context, key, mime string, size int64) (string, error)
	HGHead(ctx context.Context, key string) (int64, error)
}

// HGSealedStore 仅供后台审核与回收使用；不得将 sealed/ 的读取/删除权限交给上传签名身份。
type HGSealedStore interface {
	HGHeadSealed(ctx context.Context, key string) (int64, error)
	HGDelete(ctx context.Context, key string) error
	HGSignRead(ctx context.Context, key string) (string, error)
}

// HGSeaweedStore 管理隔离桶的 S3 客户端，调用方负责保证 Gateway 的私有访问与 TLS。
type HGSeaweedStore struct {
	bucket string            // 受信配置的隔离桶，不能从客户端请求提取。
	client *s3.Client        // 复用连接池的 S3 SDK 客户端。
	signer *s3.PresignClient // 只签发定向 PUT，禁止生成匿名公开链接。
}

// HGNewSeaweedStore 校验 S3 Gateway 地址及凭据；生产 endpoint 应为客户端可达的 TLS 域名。
func HGNewSeaweedStore(endpoint, region, bucket, accessKey, secretKey string) (*HGSeaweedStore, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || !hgBucketName.MatchString(bucket) || region == "" || accessKey == "" || secretKey == "" {
		return nil, errors.New("SeaweedFS 隔离桶配置不合法")
	}
	if u.Scheme == "http" && os.Getenv("SERVER_ENV") != "debug" {
		return nil, errors.New("非 debug 环境禁止明文媒体上传端点")
	}
	client := &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{MaxIdleConns: 256, MaxIdleConnsPerHost: 128, MaxConnsPerHost: 256, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 3 * time.Second, ResponseHeaderTimeout: 5 * time.Second}}
	sdk := s3.NewFromConfig(aws.Config{Region: region, Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""), HTTPClient: client}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(strings.TrimRight(endpoint, "/"))
		o.UsePathStyle = true
	})
	return &HGSeaweedStore{bucket: bucket, client: sdk, signer: s3.NewPresignClient(sdk)}, nil
}

// HGSignUpload 签发最长十分钟的隔离桶直传 URL；长度/类型由完成确认再核对，签名不得替代配额与审核。
func (s *HGSeaweedStore) HGSignUpload(ctx context.Context, key, mime string, size int64) (string, error) {
	if !strings.HasPrefix(key, "quarantine/") || size < 1 {
		return "", HGErrInvalid
	}
	result, err := s.signer.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(mime), ContentLength: aws.Int64(size)}, func(o *s3.PresignOptions) { o.Expires = 10 * time.Minute })
	if err != nil {
		return "", fmt.Errorf("创建媒体上传授权失败: %w", err)
	}
	// SDK 若未签入长度，必须在网关限制 PUT body；当前模块失败关闭，不发放可无限上传的 URL。
	if result.SignedHeader.Get("Content-Length") != fmt.Sprint(size) || result.SignedHeader.Get("Content-Type") != mime {
		return "", errors.New("S3 网关未签入文件大小或类型，拒绝发放上传授权")
	}
	return result.URL, nil
}

// HGHead 只检查隔离对象真实长度；不能据此判定真实 MIME、摘要或安全性。
func (s *HGSeaweedStore) HGHead(ctx context.Context, key string) (int64, error) {
	if !strings.HasPrefix(key, "quarantine/") {
		return 0, HGErrInvalid
	}
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return 0, fmt.Errorf("检查隔离对象失败: %w", err)
	}
	return aws.ToInt64(result.ContentLength), nil
}

// HGHeadSealed 在可信审核器报告成功后复查封存对象，不能仅凭回调宣称已落盘。
func (s *HGSeaweedStore) HGHeadSealed(ctx context.Context, key string) (int64, error) {
	if !hgSealedKey.MatchString(key) {
		return 0, HGErrInvalid
	}
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return 0, fmt.Errorf("复查封存对象失败: %w", err)
	}
	return aws.ToInt64(result.ContentLength), nil
}

// HGDelete 仅供持久化删除任务调用，重复删除同一 Key 视作幂等；必须由受限服务凭据执行。
func (s *HGSeaweedStore) HGDelete(ctx context.Context, key string) error {
	if !hgSealedKey.MatchString(key) && !hgQuarantineKey.MatchString(key) {
		return HGErrInvalid
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("删除媒体对象失败: %w", err)
	}
	return nil
}

// HGSignRead 仅为经过业务鉴权的封存对象签发 30 秒临时读 URL；不得对 quarantine/ 签发。
func (s *HGSeaweedStore) HGSignRead(ctx context.Context, key string) (string, error) {
	if !hgSealedKey.MatchString(key) {
		return "", HGErrInvalid
	}
	result, err := s.signer.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}, func(o *s3.PresignOptions) { o.Expires = 30 * time.Second })
	if err != nil {
		return "", fmt.Errorf("创建私有媒体读取授权失败: %w", err)
	}
	return result.URL, nil
}

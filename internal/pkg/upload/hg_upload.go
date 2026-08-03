package HGUploadPackage

import (
	"MLC_GO/internal/pkg/logHG"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// region 常量定义

const (
	// DefaultMaxFileSize 默认最大文件大小（10MB）
	DefaultMaxFileSize = 10 << 20

	// DefaultUploadDir 默认上传目录
	DefaultUploadDir = "./uploads"

	// maxDetectBytes 用于识别文件真实内容类型的最大读取字节数。
	maxDetectBytes = 512

	// 图片格式
	ImageTypeJPG  = "jpg"
	ImageTypeJPEG = "jpeg"
	ImageTypePNG  = "png"
	ImageTypeGIF  = "gif"
	ImageTypeWebP = "webp"
)

// endregion

// region 上传配置

// UploadConfig 上传配置。
type UploadConfig struct {
	MaxFileSize  int64    // 最大文件大小（字节）
	UploadDir    string   // 上传目录（本地存储时使用）
	AllowedTypes []string // 允许的图片类型
	BaseURL      string   // 基础 URL，如 https://api.example.com

	// 对象存储配置（生产环境使用）
	StorageType string // 存储类型：local / oss / s3
	OSSConfig   *OSSConfig
	S3Config    *S3Config
}

// OSSConfig 阿里云 OSS 配置。
type OSSConfig struct {
	Endpoint        string // OSS 端点
	AccessKeyID     string // AccessKey ID
	AccessKeySecret string // AccessKey Secret
	BucketName      string // Bucket 名称
	CDNDomain       string // CDN 域名（可选）
}

// S3Config configures an S3-compatible private bucket and its public CDN URL.
type S3Config struct {
	Endpoint, Region, BucketName, AccessKeyID, SecretAccessKey, CDNBaseURL string
	RequestTimeout                                                         time.Duration
}

// DefaultConfig 返回默认配置（本地存储）。
func DefaultConfig() UploadConfig {
	return UploadConfig{
		MaxFileSize: DefaultMaxFileSize,
		UploadDir:   DefaultUploadDir,
		AllowedTypes: []string{
			ImageTypeJPG,
			ImageTypeJPEG,
			ImageTypePNG,
			ImageTypeGIF,
			ImageTypeWebP,
		},
		BaseURL:     "http://localhost:8080",
		StorageType: "local",
	}
}

// endregion

// S3StorageDriver 使用 AWS Signature Version 4 访问 S3-compatible 对象存储，不引入额外 SDK。
// 当前实现会在内存中读取完整 payload 后计算 SHA-256，只适用于调用方有严格大小上限的小对象；评论图片上限为 5 MiB，不得复用于大型视频上传。
type S3StorageDriver struct {
	config S3Config
	client *http.Client
	now    func() time.Time
}

// NewS3StorageDriver 校验 S3-compatible endpoint、私有凭据和公开 CDN URL，并复用 HTTP 连接池。
func NewS3StorageDriver(config S3Config) (*S3StorageDriver, error) {
	config.Endpoint = strings.TrimRight(strings.TrimSpace(config.Endpoint), "/")
	config.CDNBaseURL = strings.TrimRight(strings.TrimSpace(config.CDNBaseURL), "/")
	if config.Endpoint == "" || config.Region == "" || config.BucketName == "" || config.AccessKeyID == "" || config.SecretAccessKey == "" || config.CDNBaseURL == "" || config.RequestTimeout <= 0 {
		return nil, fmt.Errorf("s3 storage configuration is invalid")
	}
	endpointURL, err := url.ParseRequestURI(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid s3 endpoint: %w", err)
	}
	if (endpointURL.Scheme != "http" && endpointURL.Scheme != "https") || endpointURL.Host == "" || endpointURL.User != nil {
		return nil, fmt.Errorf("invalid s3 endpoint scheme, host or userinfo")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 50
	return &S3StorageDriver{config: config, client: &http.Client{Transport: transport}, now: time.Now}, nil
}

func (d *S3StorageDriver) Upload(data []byte, key, contentType string) (string, error) {
	return d.UploadStream(bytes.NewReader(data), key, contentType)
}

func (d *S3StorageDriver) UploadStream(reader io.Reader, key, contentType string) (string, error) {
	return d.UploadStreamContext(context.Background(), reader, key, contentType)
}

func (d *S3StorageDriver) UploadStreamContext(ctx context.Context, reader io.Reader, key, contentType string) (string, error) {
	// SigV4 默认 payload 签名要求请求前得到完整哈希；这里接受有界内存开销以保持实现简单且与 S3-compatible 服务兼容。
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read s3 upload body: %w", err)
	}
	if err := d.do(ctx, http.MethodPut, key, contentType, data); err != nil {
		return "", err
	}
	return d.GetURL(key), nil
}

func (d *S3StorageDriver) Delete(key string) error { return d.DeleteContext(context.Background(), key) }

func (d *S3StorageDriver) DeleteContext(ctx context.Context, key string) error {
	return d.do(ctx, http.MethodDelete, key, "", nil)
}

func (d *S3StorageDriver) GetURL(key string) string {
	return d.config.CDNBaseURL + "/" + strings.TrimLeft(key, "/")
}

func (d *S3StorageDriver) do(ctx context.Context, method, key, contentType string, body []byte) error {
	requestCtx, cancel := context.WithTimeout(ctx, d.config.RequestTimeout)
	defer cancel()
	escapedKey := strings.ReplaceAll(url.PathEscape(strings.TrimLeft(key, "/")), "%2F", "/")
	requestURL := d.config.Endpoint + "/" + url.PathEscape(d.config.BucketName) + "/" + escapedKey
	req, err := http.NewRequestWithContext(requestCtx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create s3 request: %w", err)
	}
	now := d.now().UTC()
	payloadHash := hgSHA256Hex(body)
	// canonical request 只签名 Host、payload hash 和时间；Content-Type 不参与签名，避免代理规范化该头后造成签名不一致。
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := "host:" + req.URL.Host + "\n" + "x-amz-content-sha256:" + payloadHash + "\n" + "x-amz-date:" + req.Header.Get("X-Amz-Date") + "\n"
	canonicalRequest := method + "\n" + req.URL.EscapedPath() + "\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash
	date := now.Format("20060102")
	scope := date + "/" + d.config.Region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + req.Header.Get("X-Amz-Date") + "\n" + scope + "\n" + hgSHA256Hex([]byte(canonicalRequest))
	signingKey := hgHMAC([]byte("AWS4"+d.config.SecretAccessKey), date)
	signingKey = hgHMAC(signingKey, d.config.Region)
	signingKey = hgHMAC(signingKey, "s3")
	signingKey = hgHMAC(signingKey, "aws4_request")
	signature := hex.EncodeToString(hgHMAC(signingKey, stringToSign))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+d.config.AccessKeyID+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute s3 request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("s3 request failed: status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(errorBody)))
}

func hgSHA256Hex(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func hgHMAC(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

// region 上传结果

// UploadResult 上传结果。
type UploadResult struct {
	FileName string `json:"fileName"` // 文件名
	FilePath string `json:"filePath"` // 文件路径（本地存储时使用）
	FileURL  string `json:"fileURL"`  // 访问 URL（CDN URL）
	FileSize int64  `json:"fileSize"` // 文件大小
	IsNew    bool   `json:"isNew"`    // 是否新上传
}

// UploadTarget 是执行外部 I/O 前生成的稳定对象键和公开 URL，供调用方先持久化可恢复 reservation。
type UploadTarget struct {
	FileName string
	FilePath string
	FileURL  string
}

// endregion

// region 文件名生成器

// FileNameGenerator 文件名生成器。
type FileNameGenerator struct {
	// counter 持有一个无符号 64 位自增计数器 counter，用来提供自增序列号，确保在高并发上传时生成的文件名唯一性。使用 atomic 包的原子操作来保证线程安全。
	counter uint64
}

// globalGenerator 全局文件名生成器。
// 全局单例指针，整个程序只用这一个计数器实例，保证全局自增有序。
var globalGenerator = &FileNameGenerator{}

// GenerateFileName 生成文件名：hg_模块名+年月日时分秒+序号.图片格式
// 示例：hg_user_20260505183045123456789_000001_ab12cd34ef56abcd.jpg
func GenerateFileName(moduleName string, ext string) string {
	generator := globalGenerator

	// seq 全局唯一递增数字，每次调用 + 1
	// atomic.AddUint64：原子自增；多协程并发上传时，不加锁也能安全对 counter+1，不会出现并发重复序号。
	seq := atomic.AddUint64(&generator.counter, 1)

	// 纳秒时间、原子序号和随机后缀共同避免高并发下文件名冲突。
	now := time.Now()
	// timeStr
	// 20060102150405 Go 标准时间模板 → 年月日时分秒
	// now.Nanosecond() 9 位纳秒，补零到 9 位
	timeStr := now.Format("20060102150405") + fmt.Sprintf("%09d", now.Nanosecond())
	// randomStr 自定义函数，生成 8 位十六进制随机字符串，增加随机熵，进一步杜绝极端冲突。
	randomStr := newRandomHex(8)

	// 清理模块名
	moduleName = sanitizePathPart(moduleName)
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	// 生成文件名
	// seq%1000000： 自增数字很大（uint64 可到上亿），直接拼接字符串太长；
	// 对 1000000 取模，只保留6 位数字，缩短文件名长度，冲突概率几乎不变。%06d 不足 6 位前面补 0，保证长度统一。
	// 模板拆分：hg_{模块名}_{时间纳秒串}_{6位自增序号}_{8位随机串}.后缀
	return fmt.Sprintf("hg_%s_%s_%06d_%s.%s", moduleName, timeStr, seq%1000000, randomStr, ext)
}

// endregion

// region 存储接口

// StorageDriver 存储驱动接口。
type StorageDriver interface {
	// Upload 上传文件，返回访问 URL。
	Upload(data []byte, key string, contentType string) (string, error)
	// UploadStream 流式上传文件，返回访问 URL。
	UploadStream(reader io.Reader, key string, contentType string) (string, error)
	// Delete 删除文件。
	Delete(key string) error
	// GetURL 获取文件访问 URL。
	GetURL(key string) string
}

type contextStorageDriver interface {
	UploadStreamContext(context.Context, io.Reader, string, string) (string, error)
	DeleteContext(context.Context, string) error
}

// endregion

// region 本地存储驱动

// LocalStorageDriver 本地存储驱动（开发环境使用）。
type LocalStorageDriver struct {
	baseURL   string
	uploadDir string
}

// NewLocalStorageDriver 创建本地存储驱动。
func NewLocalStorageDriver(baseURL, uploadDir string) *LocalStorageDriver {
	return &LocalStorageDriver{
		baseURL:   baseURL,
		uploadDir: uploadDir,
	}
}

// Upload 上传文件到本地磁盘。
func (d *LocalStorageDriver) Upload(data []byte, key string, contentType string) (string, error) {
	return d.UploadStream(bytes.NewReader(data), key, contentType)
}

// UploadStream 流式上传文件到本地磁盘，避免并发上传时整文件常驻内存。
func (d *LocalStorageDriver) UploadStream(reader io.Reader, key string, contentType string) (string, error) {
	// 创建目录
	dir := filepath.Dir(filepath.Join(d.uploadDir, key))
	// 递归创建多级目录，不存在的父目录会一并创建；目录已存在时不会报错。
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create dir failed: %w", err)
	}

	// 保存文件
	// filepath.Join 拼接完整文件路径，自动适配不同系统路径分隔符（Windows \ / Mac/Linux /），避免手动拼接出现斜杠错乱。
	filePath := filepath.Join(d.uploadDir, key)
	// os.OpenFile 创建文件，如果文件已存在则返回错误。
	// os.O_WRONLY：只写；os.O_CREATE：如果文件不存在则创建；os.O_EXCL：如果文件已存在则返回错误。
	// err: 文件已存在：O_EXCL 触发 file exists 错误;目录不存在、磁盘满、权限不足等也会报错
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	// 把 reader 中的二进制图片流，完整拷贝写入本地文件 file
	if _, err := io.Copy(file, reader); err != nil { //当写入中途出错（磁盘满、流中断、IO 异常）
		// 写入失败，删除残留空/损坏文件
		_ = os.Remove(filePath) //执行 os.Remove(filePath)，把半截损坏的文件删掉，避免残留垃圾文件；
		return "", fmt.Errorf("write file failed: %w", err)
	}

	// 返回访问 URL
	return d.baseURL + "/uploads/" + key, nil
}

// Delete 删除本地文件。
func (d *LocalStorageDriver) Delete(key string) error {
	filePath := filepath.Join(d.uploadDir, key)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetURL 获取本地文件访问 URL。
func (d *LocalStorageDriver) GetURL(key string) string {
	return d.baseURL + "/uploads/" + key
}

// endregion

// region OSS 存储驱动

// OSSStorageDriver 阿里云 OSS 存储驱动（生产环境使用）。
// 注意：实际使用需要引入阿里云 OSS SDK
// go get github.com/aliyun/aliyun-oss-go-sdk/oss
type OSSStorageDriver struct {
	config *OSSConfig
}

// NewOSSStorageDriver 创建 OSS 存储驱动。
func NewOSSStorageDriver(config *OSSConfig) *OSSStorageDriver {
	return &OSSStorageDriver{config: config}
}

// Upload 上传文件到 OSS。
// 注意：这里只是示例，实际需要使用阿里云 OSS SDK
func (d *OSSStorageDriver) Upload(data []byte, key string, contentType string) (string, error) {
	return d.UploadStream(bytes.NewReader(data), key, contentType)
}

// UploadStream 上传文件流到 OSS。
// 注意：这里只是示例，实际需要使用阿里云 OSS SDK
func (d *OSSStorageDriver) UploadStream(reader io.Reader, key string, contentType string) (string, error) {
	// TODO: 实际实现需要使用阿里云 OSS SDK
	// client, err := oss.New(d.config.Endpoint, d.config.AccessKeyID, d.config.AccessKeySecret)
	// if err != nil {
	//     return "", err
	// }
	// bucket, err := client.Bucket(d.config.BucketName)
	// if err != nil {
	//     return "", err
	// }
	// err = bucket.PutObject(key, reader, oss.ContentType(contentType))
	// if err != nil {
	//     return "", err
	// }
	// return d.GetURL(key), nil

	return d.GetURL(key), nil
}

// Delete 删除 OSS 文件。
func (d *OSSStorageDriver) Delete(key string) error {
	// TODO: 实际实现需要使用阿里云 OSS SDK
	return nil
}

// GetURL 获取 OSS 文件访问 URL。
func (d *OSSStorageDriver) GetURL(key string) string {
	if d.config.CDNDomain != "" {
		return fmt.Sprintf("https://%s/%s", d.config.CDNDomain, key)
	}
	return fmt.Sprintf("https://%s.%s/%s", d.config.BucketName, d.config.Endpoint, key)
}

// endregion

// region 上传器

// Uploader 文件上传器。
type Uploader struct {
	config  UploadConfig
	storage StorageDriver
}

// NewUploader 创建上传器。
func NewUploader(config UploadConfig) *Uploader {
	var storage StorageDriver

	switch config.StorageType {
	case "s3":
		if config.S3Config == nil {
			logHG.ErrFInfo("S3 config is nil, fallback to local storage")
			storage = NewLocalStorageDriver(config.BaseURL, config.UploadDir)
		} else {
			var err error
			storage, err = NewS3StorageDriver(*config.S3Config)
			if err != nil {
				logHG.ErrFInfo("S3 config invalid, fallback to local storage: %v", err)
				storage = NewLocalStorageDriver(config.BaseURL, config.UploadDir)
			}
		}
	case "oss":
		if config.OSSConfig != nil {
			storage = NewOSSStorageDriver(config.OSSConfig)
		} else {
			logHG.ErrFInfo("OSS config is nil, fallback to local storage")
			storage = NewLocalStorageDriver(config.BaseURL, config.UploadDir)
		}
	default:
		storage = NewLocalStorageDriver(config.BaseURL, config.UploadDir)
	}

	return &Uploader{
		config:  config,
		storage: storage,
	}
}

// NewUploaderStrict rejects invalid explicitly selected storage instead of silently changing durability semantics.
// NewUploaderStrict 对显式选择的 S3 采用 fail-fast，避免生产配置错误时静默写入本地临时磁盘。
func NewUploaderStrict(config UploadConfig) (*Uploader, error) {
	if config.StorageType == "s3" {
		if config.S3Config == nil {
			return nil, fmt.Errorf("s3 config is nil")
		}
		storage, err := NewS3StorageDriver(*config.S3Config)
		if err != nil {
			return nil, err
		}
		return &Uploader{config: config, storage: storage}, nil
	}
	return NewUploader(config), nil
}

// NewDefaultUploader 创建默认上传器（本地存储）。
func NewDefaultUploader() *Uploader {
	return NewUploader(DefaultConfig())
}

// NewUploaderWithBaseURL 创建带自定义 BaseURL 的上传器（生产环境使用）。
func NewUploaderWithBaseURL(baseURL string) *Uploader {
	config := DefaultConfig()
	config.BaseURL = baseURL
	return NewUploader(config)
}

// NewOSSUploader 创建 OSS 上传器（生产环境使用）。
func NewOSSUploader(config *OSSConfig) *Uploader {
	uploadConfig := UploadConfig{
		MaxFileSize: DefaultMaxFileSize,
		AllowedTypes: []string{
			ImageTypeJPG,
			ImageTypeJPEG,
			ImageTypePNG,
			ImageTypeGIF,
			ImageTypeWebP,
		},
		StorageType: "oss",
		OSSConfig:   config,
	}
	return NewUploader(uploadConfig)
}

// UploadFromBytes 从二进制数据上传文件。
func (u *Uploader) UploadFromBytes(data []byte, moduleName string, ext string) (*UploadResult, error) {
	// 1. 检查数据大小
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}
	if int64(len(data)) > u.config.MaxFileSize {
		return nil, fmt.Errorf("data size %d exceeds max %d", len(data), u.config.MaxFileSize)
	}

	// 文件名处理
	ext = normalizeExt(ext)

	// 2. 检查文件类型
	if !u.isAllowedType(ext) {
		return nil, fmt.Errorf("file type %s not allowed", ext)
	}
	if err := validateImageContentType(data, ext); err != nil {
		return nil, err
	}

	// 3. 生成文件名和对象键
	fileName := GenerateFileName(moduleName, ext)
	moduleName = sanitizePathPart(moduleName)
	key := fmt.Sprintf("%s/%s", moduleName, fileName)

	// 4. 确定 Content-Type
	contentType := getContentType(ext)

	// 5. 上传到存储
	fileURL, err := u.storage.Upload(data, key, contentType)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	return &UploadResult{
		FileName: fileName,
		FilePath: key,
		FileURL:  fileURL,
		FileSize: int64(len(data)),
		IsNew:    true,
	}, nil
}

// UploadFromReader 从流上传文件，调用方需传入已限制大小的 reader 和准确 size。
func (u *Uploader) UploadFromReader(reader io.Reader, size int64, moduleName string, ext string) (*UploadResult, error) {
	return u.UploadFromReaderContext(context.Background(), reader, size, moduleName, ext)
}

// PrepareUploadTarget 只生成对象键和 URL，不执行存储 I/O。
func (u *Uploader) PrepareUploadTarget(moduleName, ext string) (*UploadTarget, error) {
	ext = normalizeExt(ext)
	if !u.isAllowedType(ext) {
		return nil, fmt.Errorf("file type %s not allowed", ext)
	}
	moduleName = sanitizePathPart(moduleName)
	fileName := GenerateFileName(moduleName, ext)
	key := fmt.Sprintf("%s/%s", moduleName, fileName)
	return &UploadTarget{FileName: fileName, FilePath: key, FileURL: u.storage.GetURL(key)}, nil
}

// UploadFromReaderContext 在已有大小上限和内容检测基础上，把取消信号传递给支持 context 的存储驱动。
func (u *Uploader) UploadFromReaderContext(ctx context.Context, reader io.Reader, size int64, moduleName string, ext string) (*UploadResult, error) {
	target, err := u.PrepareUploadTarget(moduleName, ext)
	if err != nil {
		return nil, err
	}
	if err := u.UploadFromReaderToKeyContext(ctx, reader, size, target.FilePath, ext); err != nil {
		return nil, err
	}
	return &UploadResult{FileName: target.FileName, FilePath: target.FilePath, FileURL: target.FileURL, FileSize: size, IsNew: true}, nil
}

// UploadFromReaderToKeyContext 将已登记 reservation 的内容写入指定对象键。
func (u *Uploader) UploadFromReaderToKeyContext(ctx context.Context, reader io.Reader, size int64, key, ext string) error {
	if reader == nil {
		return fmt.Errorf("reader is nil")
	}
	if size <= 0 {
		return fmt.Errorf("file is empty")
	}
	if size > u.config.MaxFileSize {
		return fmt.Errorf("file size %d exceeds max %d", size, u.config.MaxFileSize)
	}

	ext = normalizeExt(ext)
	if !u.isAllowedType(ext) {
		return fmt.Errorf("file type %s not allowed", ext)
	}

	detectBuf := make([]byte, maxDetectBytes)
	n, err := io.ReadFull(reader, detectBuf)
	if err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read file failed: %w", err)
		}
	}
	detectBuf = detectBuf[:n]
	if err := validateImageContentType(detectBuf, ext); err != nil {
		return err
	}

	contentType := getContentType(ext)
	counting := &hgCountingReader{reader: io.MultiReader(bytes.NewReader(detectBuf), reader)}
	stream := io.LimitReader(counting, size+1)

	if storage, ok := u.storage.(contextStorageDriver); ok {
		_, err = storage.UploadStreamContext(ctx, stream, key, contentType)
	} else {
		_, err = u.storage.UploadStream(stream, key, contentType)
	}
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	if counting.read != size {
		deleteErr := u.DeleteFileContext(context.WithoutCancel(ctx), key)
		if deleteErr != nil {
			return fmt.Errorf("uploaded size %d does not match declared size %d; delete mismatched object: %w", counting.read, size, deleteErr)
		}
		return fmt.Errorf("uploaded size %d does not match declared size %d", counting.read, size)
	}
	return nil
}

type hgCountingReader struct {
	reader io.Reader
	read   int64
}

func (r *hgCountingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += int64(n)
	return n, err
}

// UploadSingle 上传单个文件。
func (u *Uploader) UploadSingle(fileHeader *multipart.FileHeader, moduleName string) (*UploadResult, error) {
	if fileHeader == nil {
		return nil, fmt.Errorf("file header is nil")
	}
	if fileHeader.Size <= 0 {
		return nil, fmt.Errorf("file is empty")
	}
	if fileHeader.Size > u.config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds max %d", fileHeader.Size, u.config.MaxFileSize)
	}

	// 获取文件扩展名
	ext := normalizeExt(GetFileExt(fileHeader.Filename))
	if !u.isAllowedType(ext) {
		return nil, fmt.Errorf("file type %s not allowed", ext)
	}

	// 读取文件内容
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer src.Close()
	return u.UploadFromReader(src, fileHeader.Size, moduleName, ext)
}

// DeleteFile 删除文件。
func (u *Uploader) DeleteFile(key string) error {
	return u.storage.Delete(key)
}

// DeleteFileContext 通过存储 key 删除对象；S3 和本地不存在对象均按幂等成功处理。
func (u *Uploader) DeleteFileContext(ctx context.Context, key string) error {
	if storage, ok := u.storage.(contextStorageDriver); ok {
		return storage.DeleteContext(ctx, key)
	}
	return u.storage.Delete(key)
}

// isAllowedType 检查文件类型是否允许。
func (u *Uploader) isAllowedType(ext string) bool {
	for _, allowed := range u.config.AllowedTypes {
		// EqualFold 忽略大小写比较两个字符串是否相等，专门用来做不区分大小写的匹配。
		if strings.EqualFold(ext, allowed) {
			return true
		}
	}
	return false
}

// endregion

// region 辅助函数

// CheckFileExists 检查文件是否存在（本地存储时使用）。
func CheckFileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// DeleteFile 删除文件（本地存储时使用）。
func DeleteFile(filePath string) error {
	if filePath == "" {
		return nil
	}
	if !CheckFileExists(filePath) {
		return nil
	}
	return os.Remove(filePath)
}

// GetFileExt 获取文件扩展名。
func GetFileExt(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return ""
	}
	return strings.TrimPrefix(ext, ".")
}

// normalizeExt 标准化文件扩展名，避免大小写和点号造成校验绕过。【清洗文件后缀，统一转为小写、去除空格、去掉开头的点】
//
//	@param ext 后缀名
//	@return string
func normalizeExt(ext string) string {
	// strings.TrimSpace(ext) 删除字符串首尾所有空白字符，包含：空格、换行 \n、制表符 \t 都会清掉。
	// strings.TrimPrefix (上一步结果，".") 删除字符串开头的指定前缀，如果没有指定前缀，则返回原字符串。
	// strings.ToLower： 全部字母转小写，统一后缀格式，方便后续判断图片类型。
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
}

// sanitizePathPart 清理路径片段，避免模块名携带路径穿越字符。
// 清洗业务模块名，过滤 / \ : * ? " < > | 等非法路径字符，防止目录穿越、非法文件名。
//
//	@param value 模块名
//	@return string
func sanitizePathPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}

	// strings.Builder 是 Go 官方推荐的高性能字符串拼接工具，用来替代频繁 + 拼接字符串。底层是可变字节数组，减少内存拷贝，适合循环内逐字符组装字符串。
	var builder strings.Builder
	// 预分配内存，提前告知 Builder 最终大概需要多少字节空间。
	// 	len(value)：原始字符串 value 的字节长度；
	// 	Grow 会一次性申请对应容量，避免循环写入时频繁扩容、复制内存，提升性能。
	builder.Grow(len(value))
	// 遍历字符串 value 里的每一个 Unicode 字符（rune），逐个处理
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			// 把这个小写字母写入 builder 缓存，保留下来
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '_' || r == '-':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}

	cleaned := strings.Trim(builder.String(), "_")
	if cleaned == "" {
		return "default"
	}
	return cleaned
}

// validateImageContentType 校验文件真实内容类型，避免只按扩展名放行伪图片。
func validateImageContentType(data []byte, ext string) error {
	if len(data) == 0 {
		return fmt.Errorf("image data is empty")
	}

	// DetectContentType是net/http 标准库方法，根据文件二进制字节流自动识别真实 MIME 类型，不靠后缀猜，靠文件头部二进制特征判断。
	// 原理：读取字节前 512 字节，对照各类文件魔数（文件头部标识）识别： JPG 头部 FFD8FF、PNG 头部 89504E47 → image/png、WebP 头部 RIFF → image/webp
	contentType := http.DetectContentType(data)
	expected := getContentType(ext)
	if ext == ImageTypeJPG || ext == ImageTypeJPEG {
		if contentType == "image/jpeg" {
			return nil
		}
		return fmt.Errorf("image content type %s does not match extension %s", contentType, ext)
	}
	if contentType != expected {
		return fmt.Errorf("image content type %s does not match extension %s", contentType, ext)
	}
	return nil
}

// newRandomHex 生成随机十六进制后缀；随机源异常时回退到纳秒时间，避免上传主流程中断。
func newRandomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// getContentType 根据扩展名获取 Content-Type。
func getContentType(ext string) string {
	switch strings.ToLower(ext) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// endregion

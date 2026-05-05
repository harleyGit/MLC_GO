package HGUploadPackage

import (
	"MLC_GO/internal/pkg/logHG"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// region 常量定义

const (
	// DefaultMaxFileSize 默认最大文件大小（10MB）
	DefaultMaxFileSize = 10 << 20

	// DefaultUploadDir 默认上传目录
	DefaultUploadDir = "./uploads"

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
}

// OSSConfig 阿里云 OSS 配置。
type OSSConfig struct {
	Endpoint        string // OSS 端点
	AccessKeyID     string // AccessKey ID
	AccessKeySecret string // AccessKey Secret
	BucketName      string // Bucket 名称
	CDNDomain       string // CDN 域名（可选）
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

// region 上传结果

// UploadResult 上传结果。
type UploadResult struct {
	FileName string `json:"fileName"` // 文件名
	FilePath string `json:"filePath"` // 文件路径（本地存储时使用）
	FileURL  string `json:"fileURL"`  // 访问 URL（CDN URL）
	FileSize int64  `json:"fileSize"` // 文件大小
	IsNew    bool   `json:"isNew"`    // 是否新上传
}

// endregion

// region 文件名生成器

// FileNameGenerator 文件名生成器。
type FileNameGenerator struct {
	mu      sync.Mutex
	counter int64
}

// 全局文件名生成器。
var globalGenerator = &FileNameGenerator{}

// GenerateFileName 生成文件名：hg_模块名+年月日时分秒+序号.图片格式
// 示例：hg_user_20260505183045_001.jpg
func GenerateFileName(moduleName string, ext string) string {
	generator := globalGenerator
	generator.mu.Lock()
	defer generator.mu.Unlock()

	// 递增序号
	generator.counter++
	if generator.counter > 999 {
		generator.counter = 1
	}

	// 格式化时间：年月日时分秒
	now := time.Now()
	timeStr := now.Format("20060102150405")

	// 格式化序号：3位数字，补零
	seq := fmt.Sprintf("%03d", generator.counter)

	// 清理模块名
	moduleName = strings.ReplaceAll(moduleName, "/", "_")
	moduleName = strings.ReplaceAll(moduleName, "\\", "_")

	// 生成文件名
	return fmt.Sprintf("hg_%s_%s_%s.%s", moduleName, timeStr, seq, ext)
}

// endregion

// region 存储接口

// StorageDriver 存储驱动接口。
type StorageDriver interface {
	// Upload 上传文件，返回访问 URL。
	Upload(data []byte, key string, contentType string) (string, error)
	// Delete 删除文件。
	Delete(key string) error
	// GetURL 获取文件访问 URL。
	GetURL(key string) string
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
	// 创建目录
	dir := filepath.Dir(filepath.Join(d.uploadDir, key))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create dir failed: %w", err)
	}

	// 保存文件
	filePath := filepath.Join(d.uploadDir, key)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
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
	// TODO: 实际实现需要使用阿里云 OSS SDK
	// client, err := oss.New(d.config.Endpoint, d.config.AccessKeyID, d.config.AccessKeySecret)
	// if err != nil {
	//     return "", err
	// }
	// bucket, err := client.Bucket(d.config.BucketName)
	// if err != nil {
	//     return "", err
	// }
	// err = bucket.PutObject(key, bytes.NewReader(data), oss.ContentType(contentType))
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
	if int64(len(data)) > u.config.MaxFileSize {
		return nil, fmt.Errorf("data size %d exceeds max %d", len(data), u.config.MaxFileSize)
	}

	// 2. 检查文件类型
	if !u.isAllowedType(ext) {
		return nil, fmt.Errorf("file type %s not allowed", ext)
	}

	// 3. 生成文件名和对象键
	fileName := GenerateFileName(moduleName, ext)
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

// UploadSingle 上传单个文件。
func (u *Uploader) UploadSingle(fileHeader *multipart.FileHeader, moduleName string) (*UploadResult, error) {
	// 读取文件内容
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	// 获取文件扩展名
	ext := GetFileExt(fileHeader.Filename)

	return u.UploadFromBytes(data, moduleName, ext)
}

// DeleteFile 删除文件。
func (u *Uploader) DeleteFile(key string) error {
	return u.storage.Delete(key)
}

// isAllowedType 检查文件类型是否允许。
func (u *Uploader) isAllowedType(ext string) bool {
	for _, allowed := range u.config.AllowedTypes {
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

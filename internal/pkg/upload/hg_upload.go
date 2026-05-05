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
	UploadDir    string   // 上传目录
	AllowedTypes []string // 允许的图片类型
	BaseURL      string   // 基础 URL，如 https://api.example.com
}

// DefaultConfig 返回默认配置。
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
		BaseURL: "http://localhost:8080", // 默认值，生产环境应配置为 HTTPS
	}
}

// endregion

// region 上传结果

// UploadResult 上传结果。
type UploadResult struct {
	FileName string `json:"fileName"` // 文件名
	FilePath string `json:"filePath"` // 文件路径
	FileURL  string `json:"fileURL"`  // 访问 URL
	FileSize int64  `json:"fileSize"` // 文件大小
	IsNew    bool   `json:"isNew"`    // 是否新上传
	OldFile  string `json:"oldFile"`  // 旧文件路径（如果有）
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

// region 上传器

// Uploader 文件上传器。
type Uploader struct {
	config UploadConfig
}

// NewUploader 创建上传器。
func NewUploader(config UploadConfig) *Uploader {
	return &Uploader{config: config}
}

// NewDefaultUploader 创建默认上传器。
func NewDefaultUploader() *Uploader {
	return NewUploader(DefaultConfig())
}

// NewUploaderWithBaseURL 创建带自定义 BaseURL 的上传器（生产环境使用）。
func NewUploaderWithBaseURL(baseURL string) *Uploader {
	config := DefaultConfig()
	config.BaseURL = baseURL
	return NewUploader(config)
}

// UploadSingle 上传单个文件。
func (u *Uploader) UploadSingle(fileHeader *multipart.FileHeader, moduleName string) (*UploadResult, error) {
	results, err := u.UploadMultiple([]*multipart.FileHeader{fileHeader}, moduleName)
	if err != nil {
		logHG.ErrFInfo("上传文件失败: %v", err.Error())
		return nil, err
	}
	return results[0], nil
}

// UploadMultiple 上传多个文件。
func (u *Uploader) UploadMultiple(fileHeaders []*multipart.FileHeader, moduleName string) ([]*UploadResult, error) {
	if len(fileHeaders) == 0 {
		logHG.ErrFInfo("上传多个图片失败 no files provided")
		return nil, fmt.Errorf("no files provided")
	}

	results := make([]*UploadResult, 0, len(fileHeaders))

	for _, fileHeader := range fileHeaders {
		result, err := u.uploadOne(fileHeader, moduleName)
		if err != nil {
			// 上传失败，清理已上传的文件
			for _, r := range results {
				os.Remove(r.FilePath)
			}
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

// uploadOne 上传单个文件内部实现。
func (u *Uploader) uploadOne(fileHeader *multipart.FileHeader, moduleName string) (*UploadResult, error) {
	// 1. 检查文件大小
	if fileHeader.Size > u.config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds max %d", fileHeader.Size, u.config.MaxFileSize)
	}

	// 2. 检查文件类型
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileHeader.Filename), "."))
	if !u.isAllowedType(ext) {
		return nil, fmt.Errorf("file type %s not allowed", ext)
	}

	// 3. 生成文件名
	fileName := GenerateFileName(moduleName, ext)

	// 4. 创建上传目录
	uploadDir := filepath.Join(u.config.UploadDir, moduleName)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload dir failed: %w", err)
	}

	// 5. 构建文件路径
	filePath := filepath.Join(uploadDir, fileName)

	// 6. 保存文件
	if err := u.saveFile(fileHeader, filePath); err != nil {
		return nil, fmt.Errorf("save file failed: %w", err)
	}

	// 7. 构建完整访问 URL（HTTPS）
	// 格式：https://api.example.com/uploads/user/hg_user_20260505183045_001.jpg
	relativeURL := fmt.Sprintf("/uploads/%s/%s", moduleName, fileName)
	fileURL := u.config.BaseURL + relativeURL

	return &UploadResult{
		FileName: fileName,
		FilePath: filePath,
		FileURL:  fileURL,
		FileSize: fileHeader.Size,
		IsNew:    true,
	}, nil
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

// saveFile 保存文件。
func (u *Uploader) saveFile(fileHeader *multipart.FileHeader, filePath string) error {
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// endregion

// region 文件检查

// CheckFileExists 检查文件是否存在。
func CheckFileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// DeleteFile 删除文件。
func DeleteFile(filePath string) error {
	if filePath == "" {
		return nil
	}
	if !CheckFileExists(filePath) {
		return nil
	}
	return os.Remove(filePath)
}

// endregion

// region 二进制数据上传

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

	// 3. 生成文件名
	fileName := GenerateFileName(moduleName, ext)

	// 4. 创建上传目录
	uploadDir := filepath.Join(u.config.UploadDir, moduleName)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload dir failed: %w", err)
	}

	// 5. 构建文件路径
	filePath := filepath.Join(uploadDir, fileName)

	// 6. 保存文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, fmt.Errorf("save file failed: %w", err)
	}

	// 7. 构建完整访问 URL（HTTPS）
	relativeURL := fmt.Sprintf("/uploads/%s/%s", moduleName, fileName)
	fileURL := u.config.BaseURL + relativeURL

	return &UploadResult{
		FileName: fileName,
		FilePath: filePath,
		FileURL:  fileURL,
		FileSize: int64(len(data)),
		IsNew:    true,
	}, nil
}

// GetFileExt 获取文件扩展名。
func GetFileExt(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return ""
	}
	return strings.TrimPrefix(ext, ".")
}

// endregion

// region 图片去重检查

// ImageDeduplicator 图片去重器，检查图片是否已存在。
type ImageDeduplicator struct {
	uploadDir string
}

// NewImageDeduplicator 创建图片去重器。
func NewImageDeduplicator(uploadDir string) *ImageDeduplicator {
	return &ImageDeduplicator{uploadDir: uploadDir}
}

// FindExistingImage 查找已存在的图片（基于文件内容哈希）。
// 这里简化实现，基于文件名前缀查找。
func (d *ImageDeduplicator) FindExistingImage(moduleName string, userID string) (string, bool) {
	dir := filepath.Join(d.uploadDir, moduleName)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", false
	}

	// 查找用户相关的图片
	pattern := filepath.Join(dir, fmt.Sprintf("hg_%s_*", moduleName))
	matches, _ := filepath.Glob(pattern)

	// 返回第一个匹配的文件
	for _, match := range matches {
		return match, true
	}

	return "", false
}

// endregion

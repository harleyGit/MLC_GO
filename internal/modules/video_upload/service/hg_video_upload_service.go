package VideoUploadServicePackage

import (
	VideoUploadCachePackage "MLC_GO/internal/modules/video_upload/cache"
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	VideoUploadRepositoryPackage "MLC_GO/internal/modules/video_upload/repository"
	VideoUploadTaskPackage "MLC_GO/internal/modules/video_upload/task"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// maxVideoUploadSize 和前端限制保持一致，避免超大文件占满磁盘或拖垮单实例上传 goroutine。
	maxVideoUploadSize = int64(4 << 30)
	// videoUploadRoot 是当前本地上传根目录；生产环境后续可替换为对象存储实现。
	videoUploadRoot = "uploads/video"
)

var (
	// 以下错误用于 handler 精准映射 400/500，避免把用户参数问题都包装成内部错误。
	ErrVideoFileEmpty     = errors.New("视频文件为空")
	ErrVideoFileTooLarge  = errors.New("视频大小不能超过4GB")
	ErrVideoTypeInvalid   = errors.New("仅支持上传视频文件")
	ErrSubmissionInvalid  = errors.New("稿件信息不完整")
	ErrVideoConfigInvalid = errors.New("视频配置不完整")
	ErrUploadRateLimited  = errors.New("上传请求过于频繁")
	ErrSubmitDuplicated   = errors.New("稿件正在提交，请勿重复操作")
)

// Service 承载视频投稿业务编排。
// 文件保存、业务 ID 生成、基础校验放在这里，SQL 细节交给 repository。
type Service struct {
	repo      *VideoUploadRepositoryPackage.Repository
	cache     *VideoUploadCachePackage.Cache
	publisher VideoUploadTaskPackage.Publisher
}

// NewService 创建视频投稿服务。
func NewService(repo *VideoUploadRepositoryPackage.Repository, cache *VideoUploadCachePackage.Cache, publisher VideoUploadTaskPackage.Publisher) *Service {
	return &Service{repo: repo, cache: cache, publisher: publisher}
}

// UploadVideo 保存单个视频文件并写入上传完成记录。
// 多 P 投稿通过复用 submissionID 实现：第一个视频创建稿件，后续视频追加到同一 submission。
func (s *Service) UploadVideo(ctx context.Context, userID string, file io.Reader, fileName string, fileSize int64, mimeType string, submissionID string, partNumber uint32) (*VideoUploadDtoPackage.UploadVideoResponse, error) {
	if fileSize <= 0 {
		return nil, ErrVideoFileEmpty
	}
	if fileSize > maxVideoUploadSize {
		return nil, ErrVideoFileTooLarge
	}
	isVideo, fileReader := isVideoFile(fileName, mimeType, file)
	if !isVideo {
		return nil, ErrVideoTypeInvalid
	}
	file = fileReader
	if submissionID == "" {
		submissionID = newBusinessID("submission")
	} else if s.cache != nil {
		_ = s.cache.TouchUploadSession(ctx, userID, submissionID)
	}
	if partNumber == 0 {
		partNumber = 1
	}

	videoID := newBusinessID("video")
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		ext = ".mp4"
	}

	dateDir := time.Now().Format("20060102")
	storageDir := filepath.Join(videoUploadRoot, userID, dateDir)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, err
	}

	storageName := videoID + ext
	storagePath := filepath.Join(storageDir, storageName)
	dst, err := os.Create(storagePath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	// 写文件时同步计算 MD5，避免上传完成后再次读盘扫描大文件。
	hash := md5.New()
	if _, err = io.Copy(io.MultiWriter(dst, hash), file); err != nil {
		return nil, err
	}

	fileMD5 := hex.EncodeToString(hash.Sum(nil))
	fileURL := "/" + filepath.ToSlash(storagePath)
	fileTitle := trimExt(fileName)

	if err = s.repo.CreateUploadedVideo(ctx, VideoUploadRepositoryPackage.UploadedVideo{
		SubmissionID: submissionID,
		VideoID:      videoID,
		UserID:       userID,
		PartNumber:   partNumber,
		Title:        fileTitle,
		FileName:     fileName,
		FilePath:     fileURL,
		FileSize:     fileSize,
		MimeType:     mimeType,
		MD5:          fileMD5,
	}); err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.SaveUploadSession(ctx, userID, submissionID)
	}
	if s.publisher != nil {
		_ = s.publisher.Publish(ctx, VideoUploadTaskPackage.Task{
			Type:         VideoUploadTaskPackage.TaskTypeSnapshot,
			UserID:       userID,
			SubmissionID: submissionID,
			VideoID:      videoID,
			FilePath:     fileURL,
		})
	}

	return &VideoUploadDtoPackage.UploadVideoResponse{
		SubmissionID: submissionID,
		VideoID:      videoID,
		FileName:     fileName,
		FilePath:     fileURL,
		FileURL:      fileURL,
		FileSize:     fileSize,
		MimeType:     mimeType,
		MD5:          fileMD5,
		PartNumber:   partNumber,
	}, nil
}

// CheckUploadRateLimit 使用 Redis 对用户和 IP 双维度限流。
// Redis 不可用时返回错误，避免流量绕过限流直接打到上传链路和磁盘。
func (s *Service) CheckUploadRateLimit(ctx context.Context, userID string, ip string) error {
	if s.cache == nil {
		return nil
	}
	if err := s.cache.CheckUploadRateLimit(ctx, userID, ip); err != nil {
		if errors.Is(err, VideoUploadCachePackage.ErrRateLimited) {
			return ErrUploadRateLimited
		}
		return err
	}
	return nil
}

// SaveSubmission 校验并保存稿件级配置、各分 P 配置、标签、定时发布和商业推广。
func (s *Service) SaveSubmission(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest) (*VideoUploadDtoPackage.SaveSubmissionResponse, error) {
	if err := normalizeAndValidateSubmission(&req); err != nil {
		return nil, err
	}
	if err := s.acquireSubmitLock(ctx, userID, req.SubmissionID); err != nil {
		return nil, err
	}
	defer s.releaseSubmitLock(context.WithoutCancel(ctx), userID, req.SubmissionID)

	if err := s.repo.SaveSubmission(ctx, userID, req); err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.SaveSubmitResult(ctx, userID, req.SubmissionID, req.Status)
	}
	s.publishSubmissionTasks(ctx, userID, req)
	return &VideoUploadDtoPackage.SaveSubmissionResponse{
		SubmissionID: req.SubmissionID,
		Status:       req.Status,
		VideoCount:   len(req.Videos),
	}, nil
}

func (s *Service) acquireSubmitLock(ctx context.Context, userID string, submissionID string) error {
	if s.cache == nil {
		return nil
	}
	locked, err := s.cache.AcquireSubmitLock(ctx, userID, submissionID)
	if err != nil {
		return err
	}
	if !locked {
		return ErrSubmitDuplicated
	}
	return nil
}

func (s *Service) releaseSubmitLock(ctx context.Context, userID string, submissionID string) {
	if s.cache != nil {
		_ = s.cache.ReleaseSubmitLock(ctx, userID, submissionID)
	}
}

func (s *Service) publishSubmissionTasks(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest) {
	if s.publisher == nil {
		return
	}
	for _, video := range req.Videos {
		_ = s.publisher.Publish(ctx, VideoUploadTaskPackage.Task{
			Type:         VideoUploadTaskPackage.TaskTypeTranscode,
			UserID:       userID,
			SubmissionID: req.SubmissionID,
			VideoID:      video.VideoID,
		})
	}
	if req.Status == "reviewing" {
		_ = s.publisher.Publish(ctx, VideoUploadTaskPackage.Task{
			Type:         VideoUploadTaskPackage.TaskTypeAudit,
			UserID:       userID,
			SubmissionID: req.SubmissionID,
		})
	}
}

// normalizeAndValidateSubmission 做保存前的轻量业务校验和默认值补齐。
// 这里不依赖数据库，保证 handler/repository 不需要重复理解 B 站投稿表单规则。
func normalizeAndValidateSubmission(req *VideoUploadDtoPackage.SaveSubmissionRequest) error {
	if req == nil || strings.TrimSpace(req.SubmissionID) == "" || len(req.Videos) == 0 {
		return ErrSubmissionInvalid
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = strings.TrimSpace(req.Videos[0].Title)
	}
	if req.Title == "" || len([]rune(req.Title)) > 80 {
		return ErrSubmissionInvalid
	}
	if req.Category == "" {
		req.Category = req.Videos[0].Category
	}
	if req.VideoType == "" {
		req.VideoType = req.Videos[0].VideoType
	}
	if req.VideoType == "" {
		req.VideoType = "自制"
	}
	if req.Visibility == "" {
		req.Visibility = "public"
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if req.Status != "draft" && req.Status != "reviewing" {
		return ErrSubmissionInvalid
	}
	if req.DynamicDescription != "" && len([]rune(req.DynamicDescription)) > 233 {
		return ErrSubmissionInvalid
	}

	for i := range req.Videos {
		video := &req.Videos[i]
		video.VideoID = strings.TrimSpace(video.VideoID)
		video.Title = strings.TrimSpace(video.Title)
		if video.PartNumber == 0 {
			video.PartNumber = uint32(i + 1)
		}
		if video.VideoID == "" || video.Title == "" || len([]rune(video.Title)) > 80 || video.Category == "" || len(video.Tags) == 0 || len(video.Tags) > 7 {
			return ErrVideoConfigInvalid
		}
		if video.VideoType == "" {
			video.VideoType = "自制"
		}
		if video.VideoType == "转载" && strings.TrimSpace(video.SourceURL) == "" {
			return ErrVideoConfigInvalid
		}
	}

	return nil
}

// isVideoFile 通过文件头（Magic Number）和扩展名双重判断视频类型。
// 浏览器或代理可能丢失 Content-Type，因此扩展名作为兼容兜底。
// 返回值：是否为视频文件，以及可能被消费了头部数据的 reader（调用方需继续使用返回的 reader）。
func isVideoFile(fileName string, mimeType string, file io.Reader) (bool, io.Reader) {
	// 优先使用文件头检测，这是最可靠的方式
	// 读取前512字节用于内容类型检测
	buf := make([]byte, 512)
	n, err := io.ReadAtLeast(file, buf, 4) // 至少读取4字节才能有效检测
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		// 读取失败，回退到扩展名检测
		return isVideoFileByExt(fileName), file
	}
	if n == 0 {
		// 空文件，回退到扩展名检测
		return isVideoFileByExt(fileName), file
	}

	// 使用 http.DetectContentType 检测文件内容类型
	// 该函数会读取文件头中的 Magic Number 来识别文件类型
	contentType := http.DetectContentType(buf[:n])
	if strings.HasPrefix(contentType, "video/") {
		// 检测到视频类型，将读取的数据和剩余数据重新组合
		remainingReader := io.MultiReader(bytes.NewReader(buf[:n]), file)
		return true, remainingReader
	}

	// 文件头检测不是视频，回退到扩展名检测
	// 有些视频格式可能无法通过文件头识别，或者文件头被修改
	if isVideoFileByExt(fileName) {
		// 扩展名是视频格式，但文件头不是，可能是伪造的文件
		// 这里我们仍然返回 true，因为扩展名匹配
		remainingReader := io.MultiReader(bytes.NewReader(buf[:n]), file)
		return true, remainingReader
	}

	// 既不是视频文件头，也不是视频扩展名
	remainingReader := io.MultiReader(bytes.NewReader(buf[:n]), file)
	return false, remainingReader
}

// isVideoFileByExt 通过文件扩展名判断是否为视频文件。
// 作为文件头检测的补充，处理文件头无法识别的情况。
func isVideoFileByExt(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".mp4", ".mov", ".avi", ".flv", ".mkv", ".webm", ".m4v":
		return true
	default:
		return false
	}
}

// newBusinessID 生成可读业务 ID，避免把自增主键暴露给前端和跨系统调用方。
func newBusinessID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, strings.ReplaceAll(uuid.NewString(), "-", ""))
}

// trimExt 去掉文件扩展名，用作默认视频标题。
func trimExt(fileName string) string {
	ext := filepath.Ext(fileName)
	return strings.TrimSuffix(fileName, ext)
}

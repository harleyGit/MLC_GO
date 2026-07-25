package VideoUploadServicePackage

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	VideoUploadCachePackage "MLC_GO/internal/modules/video_upload/cache"
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	VideoUploadRepositoryPackage "MLC_GO/internal/modules/video_upload/repository"
	VideoUploadTaskPackage "MLC_GO/internal/modules/video_upload/task"
	HGUploadPackage "MLC_GO/internal/pkg/upload"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
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
	videoUploadRoot                = "uploads/video"
	videoStatusCounterSyncInterval = 5 * time.Minute
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
	repo      videoUploadRepository
	cache     videoUploadCache
	publisher VideoUploadTaskPackage.Publisher
	eventBus  eventPublisher
	baseURL   string // 服务基础 URL，用于拼接文件绝对访问地址
	uploader  *HGUploadPackage.Uploader
	syncer    *VideoUploadTaskPackage.StatusCounterSyncer
}

type eventPublisher interface {
	Publish(ctx context.Context, event events.DomainEvent) error
}

type videoUploadRepository interface {
	EnsureVideoListIndex(ctx context.Context) error
	CreateUploadedVideo(ctx context.Context, video VideoUploadRepositoryPackage.UploadedVideo) error
	SaveSubmission(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest) error
	SaveSubmissionWithEvents(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest, domainEvents ...events.DomainEvent) error
	GetSubmissionStatus(ctx context.Context, submissionID string, userID string) (string, bool, error)
	GetVideoListByCursor(ctx context.Context, cursor string, limit int, tagName string) ([]VideoUploadDtoPackage.VideoListItem, error)
	GetVideoStatusCounts(ctx context.Context) (map[string]int64, error)
}

// WithEventBus 注入领域事件总线。
// 业务代码只依赖 event bus 抽象，不直接依赖 Kafka producer；Kafka 不可用时发布失败不会影响主库写入结果。
func (s *Service) WithEventBus(eventBus eventPublisher) *Service {
	s.eventBus = eventBus
	return s
}

type videoUploadCache interface {
	SaveUploadSession(ctx context.Context, userID string, submissionID string) error
	TouchUploadSession(ctx context.Context, userID string, submissionID string) error
	CheckUploadRateLimit(ctx context.Context, userID string, ip string) error
	AcquireSubmitLock(ctx context.Context, userID string, submissionID string) (string, error)
	ReleaseSubmitLock(ctx context.Context, userID string, submissionID string, lockValue string) error
	SaveSubmitResult(ctx context.Context, userID string, submissionID string, status string) error
	IncrementVideoStatusCounter(ctx context.Context, status string, delta int64) error
	GetVideoStatusCounters(ctx context.Context) (map[string]int64, bool, error)
	SetVideoStatusCounters(ctx context.Context, counters map[string]int64) error
	GetVideoListPage(ctx context.Context, cursor string, pageSize int, tagName string) (*VideoUploadDtoPackage.GetVideoListResponse, bool, error)
	SetVideoListPage(ctx context.Context, cursor string, pageSize int, tagName string, resp *VideoUploadDtoPackage.GetVideoListResponse) error
	InvalidateVideoListPages(ctx context.Context) error
}

// NewService 创建视频投稿服务。
// baseURL 用于拼接文件绝对访问地址，如 http://localhost:8080。
func NewService(repo *VideoUploadRepositoryPackage.Repository, cache *VideoUploadCachePackage.Cache, publisher VideoUploadTaskPackage.Publisher, baseURL string) *Service {
	s := &Service{
		repo:      repo,
		cache:     cache,
		publisher: publisher,
		baseURL:   strings.TrimRight(baseURL, "/"),
		uploader:  HGUploadPackage.NewUploaderWithBaseURL(baseURL),
	}
	if cache != nil {
		s.syncer = VideoUploadTaskPackage.NewStatusCounterSyncer(repo, cache, videoStatusCounterSyncInterval)
	}
	return s
}

// Init 初始化服务，确保数据库索引存在。
// 应在服务启动时调用一次。
func (s *Service) Init(ctx context.Context) error {
	if err := s.repo.EnsureVideoListIndex(ctx); err != nil {
		return err
	}
	if s.syncer != nil {
		/* s.syncer.Start(...) 同步器启动函数，作用：
			1. 开启后台协程循环同步数据（数据库同步、缓存同步、消息队列消费、定时任务等）
			2.入参是 context.Context，用于控制同步器生命周期：超时、取消、关闭信号

		 WithoutCancel(ctx) 基于父 ctx 创建新的上下文，特性：
			1. 新 ctx 不会继承父 ctx 的取消信号
			2.父 ctx 调用 cancel()、父 ctx 超时、父 ctx 被关闭 → 不会影响这个新 ctx
			3.会继承父 ctx 的元数据（value）：TraceID、请求 ID、用户标识等透传
			4.新 ctx 没有手动 cancel 函数，只能依靠自身生命周期退出
		*/
		s.syncer.Start(context.WithoutCancel(ctx))
	}
	return nil
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
	// 创建 storageDir 目录及其所有不存在的父目录，目录权限设置为 rwxr-xr-x(0755)；如果创建失败（权限不足、磁盘异常等），则进入错误处理逻辑。
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
	// 非常经典的文件保存+MD5计算+文件信息生成流程
	hash := md5.New() // 创建MD5计算器，此时 hash -> 等待接受数据 -> 计算MD5值
	// 同时写文件和计算MD5，io.MultiWriter(dst, hash) 创建一个同时写入 dst 和 hash 的 writer，保证文件内容被写入磁盘的同时也被 hash 计算器接收，避免重复读取文件内容。
	if _, err = io.Copy(io.MultiWriter(dst, hash), file); err != nil {
		return nil, err
	}

	// 得到MD5值，hex.EncodeToString 将 hash.Sum(nil) 计算出的 MD5 二进制结果编码为十六进制字符串，得到最终的文件 MD5 值。
	fileMD5 := hex.EncodeToString(hash.Sum(nil))
	// 生成访问的URL，拼接完整绝对地址，可直接在浏览器访问。
	// 后续替换成对象存储 URL 时只需修改这里的生成逻辑。
	relativePath := "/" + filepath.ToSlash(storagePath)
	fileURL := s.baseURL + relativePath
	// 生成文件标题
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
	// 先补齐默认值和校验必填项，避免无效数据进入事务造成部分写入。
	if err := normalizeAndValidateSubmission(&req); err != nil {
		return nil, err
	}
	// 用户维度提交锁防止同一个稿件被重复点击提交，降低重复写库和重复事件风险。
	lockValue, err := s.acquireSubmitLock(ctx, userID, req.SubmissionID)
	if err != nil {
		return nil, err
	}
	defer s.releaseSubmitLock(context.WithoutCancel(ctx), userID, req.SubmissionID, lockValue)

	// 保存前读取旧状态，用于 Redis 计数器按 delta 更新，而不是每次 COUNT(*) 回源。
	oldStatus, _, err := s.repo.GetSubmissionStatus(ctx, req.SubmissionID, userID)
	if err != nil {
		return nil, err
	}

	// 领域事件随业务数据写入同一个 MySQL 事务的 Outbox，避免 DB 成功但 Kafka 失败导致读模型漏更新。
	domainEvents := s.submissionEvents(ctx, userID, req)
	if err := s.repo.SaveSubmissionWithEvents(ctx, userID, req, domainEvents...); err != nil {
		return nil, err
	}
	if s.cache != nil {
		// 计数器和列表页缓存属于性能优化，失败不影响主流程；后台同步器会定期从 MySQL 修正计数。
		counterStatus, counterDelta := videoListCounterUpdate(oldStatus, req.Status)
		_ = s.cache.IncrementVideoStatusCounter(ctx, counterStatus, counterDelta)
		_ = s.cache.SaveSubmitResult(ctx, userID, req.SubmissionID, req.Status)
		if oldStatus != req.Status && (isVideoListCountedStatus(oldStatus) || isVideoListCountedStatus(req.Status)) {
			_ = s.cache.InvalidateVideoListPages(ctx)
		}
	}
	// 转码/审核任务是异步副作用，当前阶段保持尽力投递，不阻断稿件保存结果。
	s.publishSubmissionTasks(ctx, userID, req)
	return &VideoUploadDtoPackage.SaveSubmissionResponse{
		SubmissionID: req.SubmissionID,
		Status:       req.Status,
		VideoCount:   len(req.Videos),
	}, nil
}

func (s *Service) publishSubmissionEvents(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest) {
	// 仅 reviewing 状态会触发审核事件；草稿保存不应该污染审核/Feed 等消费链路。
	if s.eventBus == nil || req.Status != "reviewing" {
		return
	}
	_ = s.eventBus.Publish(ctx, VideoEventsPackage.VideoReviewedEvent{
		EventMeta:    events.NewEventMeta(ctx),
		SubmissionID: req.SubmissionID,
		UserID:       userID,
	})
}

func (s *Service) submissionEvents(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest) []events.DomainEvent {
	// 只返回需要和业务事务一起提交的事件，调用方负责写入 Outbox。
	if req.Status != "reviewing" {
		return nil
	}
	return []events.DomainEvent{
		VideoEventsPackage.VideoReviewedEvent{
			EventMeta:    events.NewEventMeta(ctx),
			SubmissionID: req.SubmissionID,
			UserID:       userID,
		},
	}
}

func (s *Service) acquireSubmitLock(ctx context.Context, userID string, submissionID string) (string, error) {
	// 没有 Redis 时跳过锁，保持本地开发可用；生产环境应配置 Redis 以降低重复提交风险。
	if s.cache == nil {
		return "", nil
	}
	lockValue, err := s.cache.AcquireSubmitLock(ctx, userID, submissionID)
	if err != nil {
		return "", err
	}
	if lockValue == "" {
		return "", ErrSubmitDuplicated
	}
	return lockValue, nil
}

func (s *Service) releaseSubmitLock(ctx context.Context, userID string, submissionID string, lockValue string) {
	if s.cache != nil {
		// 用 lockValue 做安全释放，避免误删后续请求重新获得的锁。
		_ = s.cache.ReleaseSubmitLock(ctx, userID, submissionID, lockValue)
	}
}

func (s *Service) publishSubmissionTasks(ctx context.Context, userID string, req VideoUploadDtoPackage.SaveSubmissionRequest) {
	// 任务队列只做派生处理，失败由后续补偿任务兜底，不能影响稿件主事务。
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
	// 优先使用文件头检测，这是最可靠的方式.防止有些人本来是.exe文件修改成.mp4来上传
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

// GetVideoList 获取已提交审核的视频列表。
// 支持游标分页（亿级数据量优化）：首次调用传空 cursor，后续使用响应中的 nextCursor 翻页。
// total 通过 Redis 缓存（TTL 60s），避免每次请求都执行 COUNT(*) 全表扫描。
func (s *Service) GetVideoList(ctx context.Context, cursor string, pageSize int, tagName string) (*VideoUploadDtoPackage.GetVideoListResponse, error) {
	tagName = strings.TrimSpace(tagName)
	if len([]rune(tagName)) > 32 {
		return nil, errors.New("标签名称不能超过32个字符")
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if s.cache != nil {
		resp, hit, err := s.cache.GetVideoListPage(ctx, cursor, pageSize, tagName)
		if err == nil && hit {
			return resp, nil
		}
	}

	total := -1
	if tagName == "" {
		var err error
		total, err = s.getVideoListTotal(ctx)
		if err != nil {
			return nil, err
		}
	}

	if tagName == "" && total == 0 {
		resp := &VideoUploadDtoPackage.GetVideoListResponse{
			Total:    0,
			PageSize: pageSize,
			HasMore:  false,
			Videos:   []VideoUploadDtoPackage.VideoListItem{},
		}
		if s.cache != nil {
			_ = s.cache.SetVideoListPage(ctx, cursor, pageSize, tagName, resp)
		}
		return resp, nil
	}

	videos, err := s.repo.GetVideoListByCursor(ctx, cursor, pageSize+1, tagName)
	if err != nil {
		return nil, err
	}

	hasMore := len(videos) > pageSize
	if hasMore {
		videos = videos[:pageSize]
	}

	var nextCursor string
	if hasMore && len(videos) > 0 {
		nextCursor = videos[len(videos)-1].SubmitTime + "|" + videos[len(videos)-1].SubmissionID
	}

	resp := &VideoUploadDtoPackage.GetVideoListResponse{
		Total:      total,
		PageSize:   pageSize,
		HasMore:    hasMore,
		NextCursor: nextCursor,
		Videos:     videos,
	}
	if s.cache != nil {
		_ = s.cache.SetVideoListPage(ctx, cursor, pageSize, tagName, resp)
	}
	return resp, nil
}

// getVideoListTotal 获取视频总数，优先从 Redis Hash 计数器读取。
// Redis key: video_status_counter；fields: reviewing/published。HGETALL + 内存求和是 O(1) 级小 hash 访问，避免亿级表 COUNT(*) 热点。
func (s *Service) getVideoListTotal(ctx context.Context) (int, error) {
	if s.cache != nil {
		counters, hit, err := s.cache.GetVideoStatusCounters(ctx)
		if err == nil && hit {
			return videoListTotalFromCounters(counters), nil
		}
	}

	counters, err := s.repo.GetVideoStatusCounts(ctx)
	if err != nil {
		return 0, err
	}

	if s.cache != nil {
		_ = s.cache.SetVideoStatusCounters(ctx, counters)
	}

	return videoListTotalFromCounters(counters), nil
}

func videoListCounterDelta(oldStatus string, newStatus string) int64 {
	_, delta := videoListCounterUpdate(oldStatus, newStatus)
	return delta
}

func videoListCounterUpdate(oldStatus string, newStatus string) (string, int64) {
	oldVisible := isVideoListCountedStatus(oldStatus)
	newVisible := isVideoListCountedStatus(newStatus)
	if oldVisible == newVisible {
		return "", 0
	}
	if newVisible {
		return newStatus, 1
	}
	return oldStatus, -1
}

func isVideoListCountedStatus(status string) bool {
	return status == "reviewing" || status == "published"
}

func videoListTotalFromCounters(counters map[string]int64) int {
	total := counters["reviewing"] + counters["published"]
	if total < 0 {
		return 0
	}
	return int(total)
}

// SaveCoverImage 解析 base64 data URL 并保存为封面图片文件。
// 复用 HGUploadPackage.Uploader（与头像上传同一套存储驱动），返回可直接在浏览器访问的绝对 URL。
// 这段代码专门用来解析 Base64 格式的图片 DataURL，形如：data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...
/* 完整流程示例
输入 dataURL：data:image/webp;base64,UklGRigAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQAAAAA=
commaIdx 找到逗号位置
前缀是 data:、有逗号，校验通过
meta = data:image/webp;base64
raw = UklGRigAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQAAAAA=
meta 包含 image/webp → ext = "webp"
*/
func (s *Service) SaveCoverImage(ctx context.Context, userID string, dataURL string) (string, error) {
	/* strings.Index 查找字符串中第一个 , 的下标索引：
	DataURL 固定格式：元信息,base64图片内容
	逗号左边是文件类型、编码；右边是图片 base64 原文
	找不到逗号时返回 -1
	*/
	commaIdx := strings.Index(dataURL, ",")
	// commaIdx < 0：字符串里没有逗号，不是标准 dataURL
	// !strings.HasPrefix(dataURL, "data:")：字符串不以 data: 开头，根本不是 DataURL 格式
	if commaIdx < 0 || !strings.HasPrefix(dataURL, "data:") {
		return "", errors.New("无效的图片 data URL")
	}

	meta := dataURL[:commaIdx]  // 逗号前面部分，meta 是 MIME 类型描述，image/jpeg 不会走 if 判断，所以默认 ext=jpg
	raw := dataURL[commaIdx+1:] // 逗号后面全部base64字符串

	ext := "jpg" // 默认后缀jpg， 拿到 ext 后缀后，一般用来生成本地文件名，例如 uuid.${ext}，再把 raw 解码成图片文件
	if strings.Contains(meta, "image/png") {
		ext = "png"
	} else if strings.Contains(meta, "image/webp") {
		ext = "webp"
	}

	// 解码base64字符串raw为图片二进制
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", errors.New("base64 解码失败")
	}

	result, err := s.uploader.UploadFromBytes(decoded, "cover", ext)
	if err != nil {
		return "", fmt.Errorf("封面上传失败: %w", err)
	}

	return result.FileURL, nil
}

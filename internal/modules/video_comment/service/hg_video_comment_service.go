package VideoCommentServicePackage

import (
	VideoCommentCachePackage "MLC_GO/internal/modules/video_comment/cache"
	VideoCommentDtoPackage "MLC_GO/internal/modules/video_comment/dto"
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	HGUploadPackage "MLC_GO/internal/pkg/upload"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	SortLatest             = "latest"
	SortHot                = "hot"
	ReactionNone           = "none"
	ReactionLike           = "like"
	ReactionDislike        = "dislike"
	hgMaxCommentImageBytes = 5 << 20
)

var (
	ErrInvalidTarget      = errors.New("评论目标不能为空")
	ErrInvalidContent     = errors.New("评论内容长度必须为 1 到 1000 个字符")
	ErrInvalidRequestID   = errors.New("请求幂等标识不能为空")
	ErrInvalidSort        = errors.New("评论排序方式无效")
	ErrInvalidCursor      = errors.New("评论游标无效")
	ErrInvalidPageSize    = errors.New("评论分页大小必须为 1 到 50")
	ErrCommentNotFound    = errors.New("评论不存在或无权删除")
	ErrInvalidImageURLs   = errors.New("评论图片地址无效")
	ErrInvalidParent      = errors.New("父评论不存在、已删除或不属于当前视频")
	ErrInvalidReaction    = errors.New("评论反应必须为 none、like 或 dislike")
	ErrInvalidImageUpload = errors.New("评论图片必须为不超过 5 MiB 的 jpg、jpeg、png 或 webp")
	ErrCommentHasReplies  = errors.New("存在回复的评论不能删除")
	ErrImageRateLimited   = errors.New("评论图片上传过于频繁")
	ErrImageQuotaExceeded = errors.New("评论图片容量配额已用尽")
)

type hgCommentRepository interface {
	Create(context.Context, VideoCommentRepositoryPackage.HGCreateCommand) (VideoCommentRepositoryPackage.HGComment, error)
	List(context.Context, string, string, string, VideoCommentRepositoryPackage.HGListCursor, int) (VideoCommentRepositoryPackage.HGListResult, error)
	ListReplies(context.Context, string, string, VideoCommentRepositoryPackage.HGListCursor, int) (VideoCommentRepositoryPackage.HGRepliesResult, error)
	SetReaction(context.Context, string, string, string) (VideoCommentRepositoryPackage.HGReactionResult, error)
	Delete(context.Context, string, string) (bool, error)
}

type hgCommentImageUploader interface {
	Prepare(string) (HGImageUpload, error)
	UploadFromReader(context.Context, io.Reader, int64, string, string) error
	Delete(context.Context, string) error
}

type hgCommentImageGuard interface {
	Allow(context.Context, string, string) error
}
type hgCommentImageAssets interface {
	ReserveImageAsset(context.Context, VideoCommentRepositoryPackage.HGImageAsset, int64) error
	ScheduleImageCleanup(context.Context, string, string) error
}

// HGImageUpload 保留公开 URL、内部存储 key 和实际字节数，供资产登记及失败补偿使用。
type HGImageUpload struct {
	URL, StorageKey string
	SizeBytes       int64
}

// HGUploadAdapter 将共享 upload package 适配为评论图片服务依赖。
type HGUploadAdapter struct{ uploader *HGUploadPackage.Uploader }

// NewHGUploadAdapter 创建评论图片上传适配器。
func NewHGUploadAdapter(uploader *HGUploadPackage.Uploader) *HGUploadAdapter {
	return &HGUploadAdapter{uploader: uploader}
}

// Prepare 在外部 I/O 前生成稳定对象键和 URL，使数据库可先登记可恢复 reservation。
func (a *HGUploadAdapter) Prepare(ext string) (HGImageUpload, error) {
	target, err := a.uploader.PrepareUploadTarget("video_comment", ext)
	if err != nil {
		return HGImageUpload{}, err
	}
	return HGImageUpload{URL: target.FileURL, StorageKey: target.FilePath}, nil
}

// UploadFromReader 将已校验的 raw image 流写入预先登记的对象键。
func (a *HGUploadAdapter) UploadFromReader(ctx context.Context, reader io.Reader, size int64, ext, key string) error {
	return a.uploader.UploadFromReaderToKeyContext(ctx, reader, size, key, ext)
}

func (a *HGUploadAdapter) Delete(ctx context.Context, key string) error {
	return a.uploader.DeleteFileContext(ctx, key)
}

// DeleteStorageObject 让维护 worker 通过同一配置化存储驱动执行幂等对象删除。
func (a *HGUploadAdapter) DeleteStorageObject(ctx context.Context, key string) error {
	return a.Delete(ctx, key)
}

// Service 负责评论参数校验、不透明游标和作者权限流程组织。
type Service struct {
	repo               hgCommentRepository
	imageUploader      hgCommentImageUploader
	imageGuard         hgCommentImageGuard
	imageAssets        hgCommentImageAssets
	imageCapacityBytes int64
}

// NewService 创建使用指定评论仓储的业务服务。
func NewService(repo hgCommentRepository) *Service { return &Service{repo: repo} }

// NewServiceWithImageUploader 创建同时支持评论图片上传的业务服务。
func NewServiceWithImageUploader(repo hgCommentRepository, uploader hgCommentImageUploader) *Service {
	return &Service{repo: repo, imageUploader: uploader}
}

func NewServiceWithImageDependencies(repo hgCommentRepository, uploader hgCommentImageUploader, guard hgCommentImageGuard, assets hgCommentImageAssets, capacityBytes int64) *Service {
	return &Service{repo: repo, imageUploader: uploader, imageGuard: guard, imageAssets: assets, imageCapacityBytes: capacityBytes}
}

// Create 校验用户输入，并将用户维度 request_id 幂等语义交由唯一键保证。
func (s *Service) Create(ctx context.Context, userID string, req VideoCommentDtoPackage.CreateRequest) (VideoCommentDtoPackage.CommentResponse, error) {
	req.SubmissionID = strings.TrimSpace(req.SubmissionID)
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.Content = strings.TrimSpace(req.Content)
	req.ParentCommentID = strings.TrimSpace(req.ParentCommentID)
	if userID == "" || req.SubmissionID == "" {
		return VideoCommentDtoPackage.CommentResponse{}, ErrInvalidTarget
	}
	if count := utf8.RuneCountInString(req.RequestID); count < 1 || count > 64 {
		return VideoCommentDtoPackage.CommentResponse{}, ErrInvalidRequestID
	}
	imageURLs, err := hgNormalizeImageURLs(req.ImageURLs)
	if err != nil {
		if errors.Is(err, VideoCommentRepositoryPackage.ErrImageNotAvailable) {
			return VideoCommentDtoPackage.CommentResponse{}, ErrInvalidImageURLs
		}
		return VideoCommentDtoPackage.CommentResponse{}, err
	}
	if count := utf8.RuneCountInString(req.Content); count > 1000 || (count == 0 && len(imageURLs) == 0) {
		return VideoCommentDtoPackage.CommentResponse{}, ErrInvalidContent
	}
	comment, err := s.repo.Create(ctx, VideoCommentRepositoryPackage.HGCreateCommand{
		CommentID: UtilsPackage.GenerateBusinessID("CMT"), SubmissionID: req.SubmissionID,
		UserID: userID, RequestID: req.RequestID, Content: req.Content,
		ParentCommentID: req.ParentCommentID, ImageURLs: imageURLs,
	})
	if err != nil {
		return VideoCommentDtoPackage.CommentResponse{}, err
	}
	return hgResponse(comment, userID), nil
}

// List 使用 limit+1 判断是否存在下一页，避免对评论热表执行实时 COUNT。
func (s *Service) List(ctx context.Context, userID string, req VideoCommentDtoPackage.ListRequest) (VideoCommentDtoPackage.ListResponse, error) {
	req.SubmissionID = strings.TrimSpace(req.SubmissionID)
	if userID == "" || req.SubmissionID == "" {
		return VideoCommentDtoPackage.ListResponse{}, ErrInvalidTarget
	}
	if req.Sort == "" {
		req.Sort = SortLatest
	}
	if req.Sort != SortLatest && req.Sort != SortHot {
		return VideoCommentDtoPackage.ListResponse{}, ErrInvalidSort
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	if req.PageSize < 1 || req.PageSize > 50 {
		return VideoCommentDtoPackage.ListResponse{}, ErrInvalidPageSize
	}
	cursor, err := hgDecodeCursor(req.Cursor, req.Sort)
	if err != nil {
		return VideoCommentDtoPackage.ListResponse{}, err
	}
	page, err := s.repo.List(ctx, userID, req.SubmissionID, req.Sort, cursor, req.PageSize+1)
	if err != nil {
		return VideoCommentDtoPackage.ListResponse{}, err
	}
	comments := page.Comments
	hasMore := len(comments) > req.PageSize
	if hasMore {
		comments = comments[:req.PageSize]
	}
	result := VideoCommentDtoPackage.ListResponse{Comments: make([]VideoCommentDtoPackage.CommentResponse, 0, len(comments)), HasMore: hasMore, TotalCount: page.TotalCount}
	for _, comment := range comments {
		result.Comments = append(result.Comments, hgResponse(comment, userID))
	}
	if hasMore {
		result.NextCursor, err = hgEncodeCursor(req.Sort, comments[len(comments)-1])
		if err != nil {
			return VideoCommentDtoPackage.ListResponse{}, err
		}
	}
	return result, nil
}

// ListReplies 按时间正序读取回复；查询命中 reply 复合索引，游标不使用 OFFSET。
func (s *Service) ListReplies(ctx context.Context, userID string, req VideoCommentDtoPackage.RepliesRequest) (VideoCommentDtoPackage.RepliesResponse, error) {
	req.RootCommentID = strings.TrimSpace(req.RootCommentID)
	if userID == "" || req.RootCommentID == "" {
		return VideoCommentDtoPackage.RepliesResponse{}, ErrInvalidTarget
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	if req.PageSize < 1 || req.PageSize > 50 {
		return VideoCommentDtoPackage.RepliesResponse{}, ErrInvalidPageSize
	}
	cursor, err := hgDecodeCursor(req.Cursor, "replies")
	if err != nil {
		return VideoCommentDtoPackage.RepliesResponse{}, err
	}
	page, err := s.repo.ListReplies(ctx, userID, req.RootCommentID, cursor, req.PageSize+1)
	if err != nil {
		return VideoCommentDtoPackage.RepliesResponse{}, err
	}
	comments := page.Comments
	hasMore := len(comments) > req.PageSize
	if hasMore {
		comments = comments[:req.PageSize]
	}
	result := VideoCommentDtoPackage.RepliesResponse{Comments: make([]VideoCommentDtoPackage.CommentResponse, 0, len(comments)), HasMore: hasMore, TotalCount: page.TotalCount}
	for _, comment := range comments {
		result.Comments = append(result.Comments, hgResponse(comment, userID))
	}
	if hasMore {
		result.NextCursor, err = hgEncodeCursor("replies", comments[len(comments)-1])
	}
	return result, err
}

// SetReaction 执行 like/dislike/none 最终状态命令。
func (s *Service) SetReaction(ctx context.Context, userID string, req VideoCommentDtoPackage.ReactionRequest) (VideoCommentDtoPackage.ReactionResponse, error) {
	req.CommentID = strings.TrimSpace(req.CommentID)
	req.Reaction = strings.ToLower(strings.TrimSpace(req.Reaction))
	if userID == "" || req.CommentID == "" {
		return VideoCommentDtoPackage.ReactionResponse{}, ErrInvalidTarget
	}
	if req.Reaction != ReactionNone && req.Reaction != ReactionLike && req.Reaction != ReactionDislike {
		return VideoCommentDtoPackage.ReactionResponse{}, ErrInvalidReaction
	}
	result, err := s.repo.SetReaction(ctx, userID, req.CommentID, req.Reaction)
	if err != nil {
		return VideoCommentDtoPackage.ReactionResponse{}, err
	}
	return VideoCommentDtoPackage.ReactionResponse{CommentID: result.CommentID, Reaction: result.Reaction, LikeCount: result.LikeCount, DislikeCount: result.DislikeCount}, nil
}

// UploadImage 校验 raw image 边界并交给专用 5 MiB uploader 流式存储。
func (s *Service) UploadImage(ctx context.Context, userID, sourceIP string, reader io.Reader, size int64, ext string) (VideoCommentDtoPackage.ImageResponse, error) {
	ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
	if userID == "" || sourceIP == "" || size <= 0 || size > hgMaxCommentImageBytes || (ext != "jpg" && ext != "jpeg" && ext != "png" && ext != "webp") || s.imageUploader == nil || s.imageGuard == nil || s.imageAssets == nil {
		return VideoCommentDtoPackage.ImageResponse{}, ErrInvalidImageUpload
	}
	if err := s.imageGuard.Allow(ctx, userID, sourceIP); err != nil {
		if errors.Is(err, VideoCommentCachePackage.ErrImageRateLimited) {
			return VideoCommentDtoPackage.ImageResponse{}, ErrImageRateLimited
		}
		return VideoCommentDtoPackage.ImageResponse{}, err
	}
	upload, err := s.imageUploader.Prepare(ext)
	if err != nil {
		return VideoCommentDtoPackage.ImageResponse{}, err
	}
	asset := VideoCommentRepositoryPackage.HGImageAsset{ImageID: UtilsPackage.GenerateBusinessID("CIMG"), UserID: userID, StorageKey: upload.StorageKey, ImageURL: upload.URL, SizeBytes: size, ContentType: hgImageContentType(ext)}
	if err := s.imageAssets.ReserveImageAsset(ctx, asset, s.imageCapacityBytes); err != nil {
		if errors.Is(err, VideoCommentRepositoryPackage.ErrImageQuotaExceeded) {
			return VideoCommentDtoPackage.ImageResponse{}, ErrImageQuotaExceeded
		}
		return VideoCommentDtoPackage.ImageResponse{}, err
	}
	if err := s.imageUploader.UploadFromReader(ctx, reader, size, ext, upload.StorageKey); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_ = s.imageAssets.ScheduleImageCleanup(cleanupCtx, asset.ImageID, userID)
		return VideoCommentDtoPackage.ImageResponse{}, err
	}
	return VideoCommentDtoPackage.ImageResponse{ImageURL: upload.URL}, nil
}

func hgImageContentType(ext string) string {
	if ext == "jpg" || ext == "jpeg" {
		return "image/jpeg"
	}
	return "image/" + ext
}

// DetectCommentImageExt infers an allowed raw-image type without consuming bytes needed by the uploader.
func DetectCommentImageExt(reader io.Reader) (string, io.Reader, error) {
	if reader == nil {
		return "", nil, ErrInvalidImageUpload
	}
	buffer := make([]byte, 512)
	n, err := io.ReadFull(reader, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", nil, err
	}
	buffer = buffer[:n]
	var ext string
	switch http.DetectContentType(buffer) {
	case "image/jpeg":
		ext = "jpg"
	case "image/png":
		ext = "png"
	case "image/webp":
		ext = "webp"
	default:
		return "", nil, ErrInvalidImageUpload
	}
	return ext, io.MultiReader(bytes.NewReader(buffer), reader), nil
}

// Delete 将作者身份传入仓储，由单条受限 UPDATE 原子校验权限并软删除。
func (s *Service) Delete(ctx context.Context, userID string, req VideoCommentDtoPackage.DeleteRequest) (VideoCommentDtoPackage.DeleteResponse, error) {
	req.CommentID = strings.TrimSpace(req.CommentID)
	if userID == "" || req.CommentID == "" {
		return VideoCommentDtoPackage.DeleteResponse{}, ErrInvalidTarget
	}
	deleted, err := s.repo.Delete(ctx, userID, req.CommentID)
	if errors.Is(err, VideoCommentRepositoryPackage.ErrCommentHasReplies) {
		return VideoCommentDtoPackage.DeleteResponse{}, ErrCommentHasReplies
	}
	if err != nil {
		return VideoCommentDtoPackage.DeleteResponse{}, err
	}
	if !deleted {
		return VideoCommentDtoPackage.DeleteResponse{}, ErrCommentNotFound
	}
	return VideoCommentDtoPackage.DeleteResponse{Deleted: true, CommentID: req.CommentID}, nil
}

type hgCursorPayload struct {
	Sort       string `json:"s"`
	LikeCount  uint64 `json:"l,omitempty"`
	ReplyCount uint64 `json:"r,omitempty"`
	CreatedAt  int64  `json:"t"`
	ID         uint64 `json:"i"`
}

func hgEncodeCursor(sort string, comment VideoCommentRepositoryPackage.HGComment) (string, error) {
	payload, err := json.Marshal(hgCursorPayload{Sort: sort, LikeCount: comment.LikeCount, ReplyCount: comment.ReplyCount, CreatedAt: comment.CreatedAt.UnixMicro(), ID: comment.ID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func hgDecodeCursor(value string, sort string) (VideoCommentRepositoryPackage.HGListCursor, error) {
	if value == "" {
		return VideoCommentRepositoryPackage.HGListCursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return VideoCommentRepositoryPackage.HGListCursor{}, ErrInvalidCursor
	}
	var cursor hgCursorPayload
	if json.Unmarshal(payload, &cursor) != nil || cursor.Sort != sort || cursor.ID == 0 || cursor.CreatedAt <= 0 {
		return VideoCommentRepositoryPackage.HGListCursor{}, ErrInvalidCursor
	}
	return VideoCommentRepositoryPackage.HGListCursor{LikeCount: cursor.LikeCount, ReplyCount: cursor.ReplyCount, CreatedAt: time.UnixMicro(cursor.CreatedAt).UTC(), ID: cursor.ID}, nil
}

func hgResponse(comment VideoCommentRepositoryPackage.HGComment, currentUserID string) VideoCommentDtoPackage.CommentResponse {
	return VideoCommentDtoPackage.CommentResponse{
		CommentID: comment.CommentID, SubmissionID: comment.SubmissionID, UserID: comment.UserID,
		UserName: comment.UserName, AvatarURL: comment.AvatarURL, Content: comment.Content,
		LikeCount: comment.LikeCount, DislikeCount: comment.DislikeCount, ReplyCount: comment.ReplyCount,
		Reaction: comment.Reaction, ImageURLs: comment.ImageURLs, RootCommentID: comment.RootCommentID,
		ParentCommentID: comment.ParentCommentID, ReplyToUserID: comment.ReplyToUserID, ReplyToUserName: comment.ReplyToUserName,
		CreatedAt: comment.CreatedAt.UTC().Format(time.RFC3339Nano), CanDelete: comment.UserID == currentUserID,
	}
}

func hgNormalizeImageURLs(values []string) ([]string, error) {
	if len(values) > 3 {
		return nil, ErrInvalidImageURLs
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 512 {
			return nil, ErrInvalidImageURLs
		}
		parsed, err := url.Parse(value)
		if err != nil {
			return nil, ErrInvalidImageURLs
		}
		cleanedPath := path.Clean(parsed.Path)
		isLocal := parsed.Scheme == "" && parsed.Host == "" && strings.HasPrefix(cleanedPath, "/uploads/video_comment/")
		isLocalHTTP := parsed.Scheme == "http" && parsed.Host == "localhost:8080" && strings.HasPrefix(cleanedPath, "/uploads/video_comment/")
		isCDN := parsed.Scheme == "https" && parsed.Host != "" && strings.HasPrefix(cleanedPath, "/video_comment/")
		if parsed.RawQuery != "" || parsed.Fragment != "" ||
			(parsed.Scheme == "" && parsed.Host != "") ||
			cleanedPath != parsed.Path || (!isLocal && !isLocalHTTP && !isCDN) {
			return nil, ErrInvalidImageURLs
		}
		if _, ok := seen[value]; ok {
			return nil, ErrInvalidImageURLs
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

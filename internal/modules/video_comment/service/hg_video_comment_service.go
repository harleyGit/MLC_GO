package VideoCommentServicePackage

import (
	VideoCommentDtoPackage "MLC_GO/internal/modules/video_comment/dto"
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	SortLatest = "latest"
	SortHot    = "hot"
)

var (
	ErrInvalidTarget    = errors.New("评论目标不能为空")
	ErrInvalidContent   = errors.New("评论内容长度必须为 1 到 1000 个字符")
	ErrInvalidRequestID = errors.New("请求幂等标识不能为空")
	ErrInvalidSort      = errors.New("评论排序方式无效")
	ErrInvalidCursor    = errors.New("评论游标无效")
	ErrInvalidPageSize  = errors.New("评论分页大小必须为 1 到 50")
	ErrCommentNotFound  = errors.New("评论不存在或无权删除")
)

type hgCommentRepository interface {
	Create(context.Context, VideoCommentRepositoryPackage.HGCreateCommand) (VideoCommentRepositoryPackage.HGComment, error)
	List(context.Context, string, string, VideoCommentRepositoryPackage.HGListCursor, int) ([]VideoCommentRepositoryPackage.HGComment, error)
	Delete(context.Context, string, string) (bool, error)
}

// Service 负责评论参数校验、不透明游标和作者权限流程组织。
type Service struct{ repo hgCommentRepository }

// NewService 创建使用指定评论仓储的业务服务。
func NewService(repo hgCommentRepository) *Service { return &Service{repo: repo} }

// Create 校验用户输入，并将用户维度 request_id 幂等语义交由唯一键保证。
func (s *Service) Create(ctx context.Context, userID string, req VideoCommentDtoPackage.CreateRequest) (VideoCommentDtoPackage.CommentResponse, error) {
	req.SubmissionID = strings.TrimSpace(req.SubmissionID)
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.Content = strings.TrimSpace(req.Content)
	if userID == "" || req.SubmissionID == "" {
		return VideoCommentDtoPackage.CommentResponse{}, ErrInvalidTarget
	}
	if count := utf8.RuneCountInString(req.RequestID); count < 1 || count > 64 {
		return VideoCommentDtoPackage.CommentResponse{}, ErrInvalidRequestID
	}
	if count := utf8.RuneCountInString(req.Content); count < 1 || count > 1000 {
		return VideoCommentDtoPackage.CommentResponse{}, ErrInvalidContent
	}
	comment, err := s.repo.Create(ctx, VideoCommentRepositoryPackage.HGCreateCommand{
		CommentID: UtilsPackage.GenerateBusinessID("CMT"), SubmissionID: req.SubmissionID,
		UserID: userID, RequestID: req.RequestID, Content: req.Content,
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
	comments, err := s.repo.List(ctx, req.SubmissionID, req.Sort, cursor, req.PageSize+1)
	if err != nil {
		return VideoCommentDtoPackage.ListResponse{}, err
	}
	hasMore := len(comments) > req.PageSize
	if hasMore {
		comments = comments[:req.PageSize]
	}
	result := VideoCommentDtoPackage.ListResponse{Comments: make([]VideoCommentDtoPackage.CommentResponse, 0, len(comments)), HasMore: hasMore}
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

// Delete 将作者身份传入仓储，由单条受限 UPDATE 原子校验权限并软删除。
func (s *Service) Delete(ctx context.Context, userID string, req VideoCommentDtoPackage.DeleteRequest) (VideoCommentDtoPackage.DeleteResponse, error) {
	req.CommentID = strings.TrimSpace(req.CommentID)
	if userID == "" || req.CommentID == "" {
		return VideoCommentDtoPackage.DeleteResponse{}, ErrInvalidTarget
	}
	deleted, err := s.repo.Delete(ctx, userID, req.CommentID)
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
		LikeCount: comment.LikeCount, ReplyCount: comment.ReplyCount,
		CreatedAt: comment.CreatedAt.UTC().Format(time.RFC3339Nano), CanDelete: comment.UserID == currentUserID,
	}
}

package VideoCommentServicePackage

import (
	VideoCommentDtoPackage "MLC_GO/internal/modules/video_comment/dto"
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type hgFakeCommentRepository struct {
	created       VideoCommentRepositoryPackage.HGComment
	listed        VideoCommentRepositoryPackage.HGListResult
	replies       VideoCommentRepositoryPackage.HGRepliesResult
	reaction      VideoCommentRepositoryPackage.HGReactionResult
	command       VideoCommentRepositoryPackage.HGCreateCommand
	cursor        VideoCommentRepositoryPackage.HGListCursor
	reactionValue string
	limit         int
	sort          string
	deleteErr     error
}

func (f *hgFakeCommentRepository) Create(_ context.Context, command VideoCommentRepositoryPackage.HGCreateCommand) (VideoCommentRepositoryPackage.HGComment, error) {
	f.command = command
	return f.created, nil
}

func (f *hgFakeCommentRepository) List(_ context.Context, _, _ string, sort string, cursor VideoCommentRepositoryPackage.HGListCursor, limit int) (VideoCommentRepositoryPackage.HGListResult, error) {
	f.sort, f.cursor, f.limit = sort, cursor, limit
	return f.listed, nil
}

func (f *hgFakeCommentRepository) ListReplies(_ context.Context, _, _ string, cursor VideoCommentRepositoryPackage.HGListCursor, limit int) (VideoCommentRepositoryPackage.HGRepliesResult, error) {
	f.cursor, f.limit = cursor, limit
	return f.replies, nil
}

func (f *hgFakeCommentRepository) SetReaction(_ context.Context, _, _ string, reaction string) (VideoCommentRepositoryPackage.HGReactionResult, error) {
	f.reactionValue = reaction
	return f.reaction, nil
}

func (f *hgFakeCommentRepository) Delete(context.Context, string, string) (bool, error) {
	return true, f.deleteErr
}

func TestCreateTrimsContentAndRequiresRequestID(t *testing.T) {
	repo := &hgFakeCommentRepository{created: VideoCommentRepositoryPackage.HGComment{CommentID: "CMT_1"}}
	service := NewService(repo)

	_, err := service.Create(context.Background(), "user-1", VideoCommentDtoPackage.CreateRequest{
		SubmissionID: "submission-1", Content: "  hello  ", RequestID: "request-1",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repo.command.Content != "hello" || repo.command.RequestID != "request-1" || !strings.HasPrefix(repo.command.CommentID, "CMT_") {
		t.Fatalf("Create() command = %+v", repo.command)
	}

	_, err = service.Create(context.Background(), "user-1", VideoCommentDtoPackage.CreateRequest{SubmissionID: "submission-1", Content: "hello"})
	if !errors.Is(err, ErrInvalidRequestID) {
		t.Fatalf("Create() error = %v, want ErrInvalidRequestID", err)
	}
}

func TestCreateRejectsContentOutsideRuneLimit(t *testing.T) {
	service := NewService(&hgFakeCommentRepository{})
	for _, content := range []string{"   ", strings.Repeat("界", 1001)} {
		_, err := service.Create(context.Background(), "user-1", VideoCommentDtoPackage.CreateRequest{
			SubmissionID: "submission-1", Content: content, RequestID: "request-1",
		})
		if !errors.Is(err, ErrInvalidContent) {
			t.Fatalf("Create(content length %d) error = %v", len([]rune(content)), err)
		}
	}
}

func TestCreateAllowsImagesWithoutContentAndNormalizesReplyInput(t *testing.T) {
	repo := &hgFakeCommentRepository{created: VideoCommentRepositoryPackage.HGComment{
		CommentID: "CMT_2", ParentCommentID: "CMT_1", RootCommentID: "CMT_1", ReplyToUserID: "user-2",
		ImageURLs: []string{"/uploads/video_comment/a.png"}, Reaction: ReactionNone,
	}}
	service := NewService(repo)

	result, err := service.Create(context.Background(), "user-1", VideoCommentDtoPackage.CreateRequest{
		SubmissionID: "submission-1", RequestID: "request-2", ParentCommentID: "  CMT_1  ",
		ImageURLs: []string{"  http://localhost:8080/uploads/video_comment/a.png  "},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repo.command.ParentCommentID != "CMT_1" || repo.command.Content != "" {
		t.Fatalf("Create() command = %+v", repo.command)
	}
	if len(repo.command.ImageURLs) != 1 || repo.command.ImageURLs[0] != "http://localhost:8080/uploads/video_comment/a.png" {
		t.Fatalf("Create() image URLs = %#v", repo.command.ImageURLs)
	}
	if result.ParentCommentID != "CMT_1" || result.RootCommentID != "CMT_1" || result.ReplyToUserID != "user-2" {
		t.Fatalf("Create() result = %+v", result)
	}
}

func TestCreateRejectsInvalidImageURLs(t *testing.T) {
	service := NewService(&hgFakeCommentRepository{})
	tests := [][]string{
		{"https://example.com/uploads/video_comment/a.png"},
		{"//evil.example/uploads/video_comment/a.png"},
		{"/uploads/video_comment/a.png?token=1"},
		{"/uploads/video_comment/../secret.png"},
		{"/uploads/video_comment/a/../../secret.png"},
		{"/uploads/video_comment//a.png"},
		{"/uploads/video_comment/a.png", "/uploads/video_comment/a.png"},
		{"/uploads/video_comment/1.png", "/uploads/video_comment/2.png", "/uploads/video_comment/3.png", "/uploads/video_comment/4.png"},
		{strings.Repeat("x", 513)},
	}
	for _, imageURLs := range tests {
		_, err := service.Create(context.Background(), "user-1", VideoCommentDtoPackage.CreateRequest{
			SubmissionID: "submission-1", RequestID: "request-1", ImageURLs: imageURLs,
		})
		if !errors.Is(err, ErrInvalidImageURLs) {
			t.Fatalf("Create(%q) error = %v, want ErrInvalidImageURLs", imageURLs, err)
		}
	}
}

func TestListDefaultsAndRoundTripsOpaqueLatestCursor(t *testing.T) {
	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	listed := make([]VideoCommentRepositoryPackage.HGComment, 0, 21)
	for id := uint64(21); id > 0; id-- {
		listed = append(listed, VideoCommentRepositoryPackage.HGComment{ID: id, CommentID: "CMT", CreatedAt: createdAt.Add(-time.Duration(21-id) * time.Second)})
	}
	repo := &hgFakeCommentRepository{listed: VideoCommentRepositoryPackage.HGListResult{Comments: listed, TotalCount: 45}}
	service := NewService(repo)

	result, err := service.List(context.Background(), "user-1", VideoCommentDtoPackage.ListRequest{SubmissionID: "submission-1"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.sort != SortLatest || repo.limit != 21 || !result.HasMore || result.NextCursor == "" || result.TotalCount != 45 || strings.Contains(result.NextCursor, "CMT_") {
		t.Fatalf("List() result = %+v, sort=%s limit=%d", result, repo.sort, repo.limit)
	}

	repo.listed = VideoCommentRepositoryPackage.HGListResult{}
	_, err = service.List(context.Background(), "user-1", VideoCommentDtoPackage.ListRequest{
		SubmissionID: "submission-1", Sort: SortLatest, Cursor: result.NextCursor, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("List(cursor) error = %v", err)
	}
	if repo.limit != 51 || repo.cursor.ID != 2 || !repo.cursor.CreatedAt.Equal(createdAt.Add(-19*time.Second)) {
		t.Fatalf("decoded cursor = %+v, limit=%d", repo.cursor, repo.limit)
	}
}

func TestRepliesUseChronologicalCursorAndRootReplyCount(t *testing.T) {
	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	repo := &hgFakeCommentRepository{replies: VideoCommentRepositoryPackage.HGRepliesResult{
		Comments: []VideoCommentRepositoryPackage.HGComment{
			{ID: 1, CommentID: "CMT_1", CreatedAt: createdAt},
			{ID: 2, CommentID: "CMT_2", CreatedAt: createdAt.Add(time.Second)},
			{ID: 3, CommentID: "CMT_3", CreatedAt: createdAt.Add(2 * time.Second)},
		},
		TotalCount: 9,
	}}
	service := NewService(repo)

	result, err := service.ListReplies(context.Background(), "user-1", VideoCommentDtoPackage.RepliesRequest{RootCommentID: "CMT_ROOT", PageSize: 2})
	if err != nil {
		t.Fatalf("ListReplies() error = %v", err)
	}
	if !result.HasMore || result.TotalCount != 9 || result.NextCursor == "" || len(result.Comments) != 2 {
		t.Fatalf("ListReplies() result = %+v", result)
	}

	repo.replies = VideoCommentRepositoryPackage.HGRepliesResult{}
	_, err = service.ListReplies(context.Background(), "user-1", VideoCommentDtoPackage.RepliesRequest{
		RootCommentID: "CMT_ROOT", Cursor: result.NextCursor, PageSize: 50,
	})
	if err != nil || repo.limit != 51 || repo.cursor.ID != 2 || !repo.cursor.CreatedAt.Equal(createdAt.Add(time.Second)) {
		t.Fatalf("ListReplies(cursor) error=%v cursor=%+v limit=%d", err, repo.cursor, repo.limit)
	}
}

func TestReactionValidatesFinalStateAndReturnsCounters(t *testing.T) {
	repo := &hgFakeCommentRepository{reaction: VideoCommentRepositoryPackage.HGReactionResult{
		CommentID: "CMT_1", Reaction: ReactionDislike, LikeCount: 7, DislikeCount: 2,
	}}
	service := NewService(repo)

	result, err := service.SetReaction(context.Background(), "user-1", VideoCommentDtoPackage.ReactionRequest{CommentID: " CMT_1 ", Reaction: ReactionDislike})
	if err != nil {
		t.Fatalf("SetReaction() error = %v", err)
	}
	if repo.reactionValue != ReactionDislike || result.DislikeCount != 2 || result.Reaction != ReactionDislike {
		t.Fatalf("SetReaction() result=%+v repository reaction=%q", result, repo.reactionValue)
	}
	if _, err := service.SetReaction(context.Background(), "user-1", VideoCommentDtoPackage.ReactionRequest{CommentID: "CMT_1", Reaction: "love"}); !errors.Is(err, ErrInvalidReaction) {
		t.Fatalf("SetReaction(love) error = %v, want ErrInvalidReaction", err)
	}
}

func TestDeleteMapsRepositoryRootReplyConflict(t *testing.T) {
	service := NewService(&hgFakeCommentRepository{deleteErr: VideoCommentRepositoryPackage.ErrCommentHasReplies})
	_, err := service.Delete(context.Background(), "user-1", VideoCommentDtoPackage.DeleteRequest{CommentID: "CMT_ROOT"})
	if !errors.Is(err, ErrCommentHasReplies) {
		t.Fatalf("Delete() error=%v, want ErrCommentHasReplies", err)
	}
}

func TestCreateRejectsOversizedRequestID(t *testing.T) {
	service := NewService(&hgFakeCommentRepository{})
	_, err := service.Create(context.Background(), "user-1", VideoCommentDtoPackage.CreateRequest{
		SubmissionID: "submission-1", Content: "hello", RequestID: strings.Repeat("r", 65),
	})
	if !errors.Is(err, ErrInvalidRequestID) {
		t.Fatalf("Create() error = %v, want ErrInvalidRequestID", err)
	}
}

func TestListRejectsInvalidSortCursorAndPageSize(t *testing.T) {
	service := NewService(&hgFakeCommentRepository{})
	tests := []VideoCommentDtoPackage.ListRequest{
		{SubmissionID: "submission-1", Sort: "oldest"},
		{SubmissionID: "submission-1", Sort: SortHot, Cursor: "not-a-cursor"},
		{SubmissionID: "submission-1", PageSize: 51},
	}
	for _, req := range tests {
		if _, err := service.List(context.Background(), "user-1", req); err == nil {
			t.Fatalf("List(%+v) expected error", req)
		}
	}
}

func TestResponsesMatchFrontendCommentContract(t *testing.T) {
	listJSON, err := json.Marshal(VideoCommentDtoPackage.ListResponse{Comments: []VideoCommentDtoPackage.CommentResponse{}})
	if err != nil {
		t.Fatal(err)
	}
	if string(listJSON) != `{"comments":[],"hasMore":false,"totalCount":0}` {
		t.Fatalf("list response JSON = %s", listJSON)
	}

	deleteJSON, err := json.Marshal(VideoCommentDtoPackage.DeleteResponse{Deleted: true, CommentID: "CMT_1"})
	if err != nil {
		t.Fatal(err)
	}
	if string(deleteJSON) != `{"deleted":true,"commentId":"CMT_1"}` {
		t.Fatalf("delete response JSON = %s", deleteJSON)
	}
}

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
	created VideoCommentRepositoryPackage.HGComment
	listed  []VideoCommentRepositoryPackage.HGComment
	command VideoCommentRepositoryPackage.HGCreateCommand
	cursor  VideoCommentRepositoryPackage.HGListCursor
	limit   int
	sort    string
}

func (f *hgFakeCommentRepository) Create(_ context.Context, command VideoCommentRepositoryPackage.HGCreateCommand) (VideoCommentRepositoryPackage.HGComment, error) {
	f.command = command
	return f.created, nil
}

func (f *hgFakeCommentRepository) List(_ context.Context, _ string, sort string, cursor VideoCommentRepositoryPackage.HGListCursor, limit int) ([]VideoCommentRepositoryPackage.HGComment, error) {
	f.sort, f.cursor, f.limit = sort, cursor, limit
	return f.listed, nil
}

func (f *hgFakeCommentRepository) Delete(context.Context, string, string) (bool, error) {
	return true, nil
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

func TestListDefaultsAndRoundTripsOpaqueLatestCursor(t *testing.T) {
	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	listed := make([]VideoCommentRepositoryPackage.HGComment, 0, 21)
	for id := uint64(21); id > 0; id-- {
		listed = append(listed, VideoCommentRepositoryPackage.HGComment{ID: id, CommentID: "CMT", CreatedAt: createdAt.Add(-time.Duration(21-id) * time.Second)})
	}
	repo := &hgFakeCommentRepository{listed: listed}
	service := NewService(repo)

	result, err := service.List(context.Background(), "user-1", VideoCommentDtoPackage.ListRequest{SubmissionID: "submission-1"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.sort != SortLatest || repo.limit != 21 || !result.HasMore || result.NextCursor == "" || strings.Contains(result.NextCursor, "CMT_") {
		t.Fatalf("List() result = %+v, sort=%s limit=%d", result, repo.sort, repo.limit)
	}

	repo.listed = nil
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
	if string(listJSON) != `{"comments":[],"hasMore":false}` {
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

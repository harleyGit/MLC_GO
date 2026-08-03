package VideoCommentRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreateVerifiesSubmissionAndWritesCommentInOneTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("submission-1", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_1", "submission-1", "user-1", "request-1", "hello").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("CMT_1").WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-1", "alice", "/a.png", "hello", 0, 0, createdAt))
	mock.ExpectCommit()

	repo := NewRepository(db)
	comment, err := repo.Create(context.Background(), HGCreateCommand{
		CommentID: "CMT_1", SubmissionID: "submission-1", UserID: "user-1", RequestID: "request-1", Content: "hello",
	})
	if err != nil || comment.CommentID != "CMT_1" {
		t.Fatalf("Create() comment=%+v error=%v", comment, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCreateResolvesVideoIDBeforeWritingComment(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("video-1", "video-1").
		WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_1", "submission-1", "user-1", "request-1", "hello").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("CMT_1").
		WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-1", "alice", "/a.png", "hello", 0, 0, createdAt))
	mock.ExpectCommit()

	comment, err := NewRepository(db).Create(context.Background(), HGCreateCommand{
		CommentID: "CMT_1", SubmissionID: "video-1", UserID: "user-1", RequestID: "request-1", Content: "hello",
	})
	if err != nil || comment.SubmissionID != "submission-1" {
		t.Fatalf("Create() comment=%+v error=%v", comment, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCommentableSubmissionMatchesVisibleVideoStatuses(t *testing.T) {
	query := SQLQueriesPackage.SelectCommentableSubmissionSQL
	if !strings.Contains(query, "status IN ('reviewing', 'published')") {
		t.Fatalf("SelectCommentableSubmissionSQL = %q, want reviewing and published statuses", query)
	}
	if !strings.Contains(query, "video_files") || !strings.Contains(query, "video_id = ?") {
		t.Fatalf("SelectCommentableSubmissionSQL = %q, want video_id resolution", query)
	}
}

func TestListHotUsesFullKeysetBeforeLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListVideoCommentsHotByCursorSQL)).
		WithArgs("submission-1", uint64(12), uint64(3), createdAt, uint64(99), 21).
		WillReturnRows(hgCommentRows(createdAt))

	repo := NewRepository(db)
	_, err = repo.List(context.Background(), "submission-1", "hot", HGListCursor{
		LikeCount: 12, ReplyCount: 3, CreatedAt: createdAt, ID: 99,
	}, 21)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestDeleteIsAuthorScopedSoftDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.SoftDeleteVideoCommentSQL)).
		WithArgs("CMT_1", "user-1").WillReturnResult(sqlmock.NewResult(0, 1))

	deleted, err := NewRepository(db).Delete(context.Background(), "user-1", "CMT_1")
	if err != nil || !deleted {
		t.Fatalf("Delete() deleted=%v error=%v", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func hgCommentRows(_ time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "comment_id", "submission_id", "user_id", "user_name", "avatar_url", "content", "like_count", "reply_count", "created_at"})
}

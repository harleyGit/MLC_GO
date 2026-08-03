package VideoCommentRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
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
		WithArgs("CMT_1", "submission-1", "user-1", "request-1", nil, nil, nil, "hello", `[]`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("user-1", "CMT_1").WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-1", "alice", "/a.png", "hello", nil, nil, nil, 0, 0, 0, "none", `[]`, createdAt))
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
		WithArgs("CMT_1", "submission-1", "user-1", "request-1", nil, nil, nil, "hello", `[]`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("user-1", "CMT_1").
		WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-1", "alice", "/a.png", "hello", nil, nil, nil, 0, 0, 0, "none", `[]`, createdAt))
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
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("submission-1", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListVideoCommentsHotByCursorSQL)).
		WithArgs("user-1", "submission-1", uint64(12), uint64(3), createdAt, uint64(99), 21).
		WillReturnRows(hgCommentRows(createdAt))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentTotalCountSQL)).
		WithArgs("submission-1").WillReturnRows(sqlmock.NewRows([]string{"total_count"}).AddRow(0))

	repo := NewRepository(db)
	_, err = repo.List(context.Background(), "user-1", "submission-1", "hot", HGListCursor{
		LikeCount: 12, ReplyCount: 3, CreatedAt: createdAt, ID: 99,
	}, 21)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestListResolvesVideoIDBeforeLatestQueryAndTotal(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("video-1", "video-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListVideoCommentsLatestFirstSQL)).
		WithArgs("user-1", "submission-1", 21).
		WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-2", "alice", "/a.png", "hello", nil, nil, nil, 0, 0, 0, "none", `[]`, createdAt))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentTotalCountSQL)).
		WithArgs("submission-1").WillReturnRows(sqlmock.NewRows([]string{"total_count"}).AddRow(3))

	result, err := NewRepository(db).List(context.Background(), "user-1", "video-1", "latest", HGListCursor{}, 21)
	if err != nil || result.TotalCount != 3 || len(result.Comments) != 1 {
		t.Fatalf("List() result=%+v error=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestListRejectsNonCommentableTargetConsistently(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("video-hidden", "video-hidden").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}))

	_, err = NewRepository(db).List(context.Background(), "user-1", "video-hidden", "latest", HGListCursor{}, 21)
	if !errors.Is(err, ErrSubmissionNotCommentable) {
		t.Fatalf("List() error=%v, want ErrSubmissionNotCommentable", err)
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

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentDeleteTargetForUpdateSQL)).
		WithArgs("CMT_1", "user-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id", "root_comment_id", "reply_count"}).AddRow("submission-1", nil, 0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.SoftDeleteVideoCommentSQL)).
		WithArgs("CMT_1", "user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DecrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.MarkVideoCommentImagesDeletePendingSQL)).
		WithArgs("CMT_1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	deleted, err := NewRepository(db).Delete(context.Background(), "user-1", "CMT_1")
	if err != nil || !deleted {
		t.Fatalf("Delete() deleted=%v error=%v", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCreateReplyDerivesThreadFieldsAndUpdatesShardAndRoot(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByRequestIDSQL)).WithArgs("user-1", "user-1", "request-1").WillReturnRows(hgCommentRows(createdAt))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("submission-1", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentParentForUpdateSQL)).
		WithArgs("CMT_PARENT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id", "root_comment_id", "user_id"}).AddRow("CMT_PARENT", "CMT_ROOT", "user-2"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentRootForUpdateSQL)).
		WithArgs("CMT_ROOT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id"}).AddRow("CMT_ROOT"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentImageForAttachSQL)).
		WithArgs("/uploads/video_comment/a.png", "/uploads/video_comment/a.png", "user-1").WillReturnRows(sqlmock.NewRows([]string{"image_id"}).AddRow("CIMG_1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_REPLY", "submission-1", "user-1", "request-1", "CMT_ROOT", "CMT_PARENT", "user-2", "hello", `["/uploads/video_comment/a.png"]`).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.AttachVideoCommentImageSQL)).
		WithArgs("CMT_REPLY", "CIMG_1", "user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentReplyCountSQL)).
		WithArgs("CMT_ROOT").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("user-1", "CMT_REPLY").WillReturnRows(hgCommentRows(createdAt).AddRow(2, "CMT_REPLY", "submission-1", "user-1", "alice", "/a.png", "hello", "CMT_ROOT", "CMT_PARENT", "user-2", 0, 0, 0, "none", `["/uploads/video_comment/a.png"]`, createdAt))
	mock.ExpectCommit()

	comment, err := NewRepository(db).Create(context.Background(), HGCreateCommand{
		CommentID: "CMT_REPLY", SubmissionID: "submission-1", UserID: "user-1", RequestID: "request-1",
		ParentCommentID: "CMT_PARENT", Content: "hello", ImageURLs: []string{"/uploads/video_comment/a.png"},
	})
	if err != nil || comment.RootCommentID != "CMT_ROOT" || comment.ReplyToUserID != "user-2" {
		t.Fatalf("Create() comment=%+v error=%v", comment, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCreateNestedReplyLocksParentThenVisibleRootAndChecksCounterRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("submission-1", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentParentForUpdateSQL)).
		WithArgs("CMT_PARENT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id", "root_comment_id", "user_id"}).AddRow("CMT_PARENT", "CMT_ROOT", "user-2"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentRootForUpdateSQL)).
		WithArgs("CMT_ROOT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id"}).AddRow("CMT_ROOT"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_REPLY", "submission-1", "user-1", "request-1", "CMT_ROOT", "CMT_PARENT", "user-2", "hello", `[]`).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentReplyCountSQL)).
		WithArgs("CMT_ROOT").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("user-1", "CMT_REPLY").WillReturnRows(hgCommentRows(createdAt).AddRow(2, "CMT_REPLY", "submission-1", "user-1", "alice", "/a.png", "hello", "CMT_ROOT", "CMT_PARENT", "user-2", 0, 0, 0, "none", `[]`, createdAt))
	mock.ExpectCommit()

	comment, err := NewRepository(db).Create(context.Background(), HGCreateCommand{
		CommentID: "CMT_REPLY", SubmissionID: "submission-1", UserID: "user-1", RequestID: "request-1", ParentCommentID: "CMT_PARENT", Content: "hello",
	})
	if err != nil || comment.RootCommentID != "CMT_ROOT" {
		t.Fatalf("Create() comment=%+v error=%v", comment, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCreateDirectRootReplyLocksRootOnlyOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("submission-1", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentParentForUpdateSQL)).
		WithArgs("CMT_ROOT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id", "root_comment_id", "user_id"}).AddRow("CMT_ROOT", nil, "user-2"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_REPLY", "submission-1", "user-1", "request-1", "CMT_ROOT", "CMT_ROOT", "user-2", "hello", `[]`).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentReplyCountSQL)).
		WithArgs("CMT_ROOT").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("user-1", "CMT_REPLY").WillReturnRows(hgCommentRows(createdAt).AddRow(2, "CMT_REPLY", "submission-1", "user-1", "alice", "/a.png", "hello", "CMT_ROOT", "CMT_ROOT", "user-2", 0, 0, 0, "none", `[]`, createdAt))
	mock.ExpectCommit()

	_, err = NewRepository(db).Create(context.Background(), HGCreateCommand{
		CommentID: "CMT_REPLY", SubmissionID: "submission-1", UserID: "user-1", RequestID: "request-1", ParentCommentID: "CMT_ROOT", Content: "hello",
	})
	if err != nil {
		t.Fatalf("Create() error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCreateRejectsMissingCounterMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("submission-1", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_1", "submission-1", "user-1", "request-1", nil, nil, nil, "hello", `[]`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err = NewRepository(db).Create(context.Background(), HGCreateCommand{
		CommentID: "CMT_1", SubmissionID: "submission-1", UserID: "user-1", RequestID: "request-1", Content: "hello",
	})
	if err == nil {
		t.Fatal("Create() expected counter consistency error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCreateReplyRejectsMissingRootCounterMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("submission-1", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentParentForUpdateSQL)).
		WithArgs("CMT_ROOT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id", "root_comment_id", "user_id"}).AddRow("CMT_ROOT", nil, "user-2"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_REPLY", "submission-1", "user-1", "request-1", "CMT_ROOT", "CMT_ROOT", "user-2", "hello", `[]`).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentReplyCountSQL)).
		WithArgs("CMT_ROOT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err = NewRepository(db).Create(context.Background(), HGCreateCommand{
		CommentID: "CMT_REPLY", SubmissionID: "submission-1", UserID: "user-1", RequestID: "request-1", ParentCommentID: "CMT_ROOT", Content: "hello",
	})
	if !errors.Is(err, ErrCounterConsistency) {
		t.Fatalf("Create() error=%v, want ErrCounterConsistency", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestListJoinsViewerReactionAndReadsShardedTotal(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).
		WithArgs("submission-1", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListVideoCommentsLatestFirstSQL)).
		WithArgs("user-1", "submission-1", 21).
		WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-2", "alice", "/a.png", "hello", nil, nil, nil, 4, 1, 2, "dislike", `[]`, createdAt))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentTotalCountSQL)).
		WithArgs("submission-1").WillReturnRows(sqlmock.NewRows([]string{"total_count"}).AddRow(12))

	result, err := NewRepository(db).List(context.Background(), "user-1", "submission-1", "latest", HGListCursor{}, 21)
	if err != nil || result.TotalCount != 12 || len(result.Comments) != 1 || result.Comments[0].Reaction != "dislike" {
		t.Fatalf("List() result=%+v error=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestListRepliesUsesSubmissionLeadingReplyIndexAndRootCounter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentRootListMetadataSQL)).
		WithArgs("CMT_ROOT").WillReturnRows(sqlmock.NewRows([]string{"submission_id", "reply_count"}).AddRow("submission-1", 4))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListVideoCommentRepliesFirstSQL)).
		WithArgs("user-1", "submission-1", "CMT_ROOT", 21).
		WillReturnRows(hgCommentRows(createdAt).AddRow(2, "CMT_REPLY", "submission-1", "user-2", "alice", "/a.png", "reply", "CMT_ROOT", "CMT_ROOT", "user-3", 0, 0, 0, "none", `[]`, createdAt))

	result, err := NewRepository(db).ListReplies(context.Background(), "user-1", "CMT_ROOT", HGListCursor{}, 21)
	if err != nil || result.TotalCount != 4 || len(result.Comments) != 1 {
		t.Fatalf("ListReplies() result=%+v error=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSetReactionIsIdempotentFinalStateTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionTargetSQL)).
		WithArgs("CMT_1").WillReturnRows(sqlmock.NewRows([]string{"comment_id"}).AddRow("CMT_1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureVideoCommentReactionSQL)).
		WithArgs("CMT_1", "user-1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionForUpdateSQL)).
		WithArgs("CMT_1", "user-1").WillReturnRows(sqlmock.NewRows([]string{"reaction"}).AddRow("dislike"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionShardTotalsSQL)).
		WithArgs("CMT_1").WillReturnRows(sqlmock.NewRows([]string{"like_count", "dislike_count"}).AddRow(7, 2))
	mock.ExpectCommit()

	result, err := NewRepository(db).SetReaction(context.Background(), "user-1", "CMT_1", "dislike")
	if err != nil || result.LikeCount != 7 || result.DislikeCount != 2 || result.Reaction != "dislike" {
		t.Fatalf("SetReaction() result=%+v error=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSetReactionUpdatesOneShardInsteadOfCommentHotRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionTargetSQL)).WithArgs("CMT_1").WillReturnRows(sqlmock.NewRows([]string{"comment_id"}).AddRow("CMT_1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureVideoCommentReactionSQL)).WithArgs("CMT_1", "user-1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionForUpdateSQL)).WithArgs("CMT_1", "user-1").WillReturnRows(sqlmock.NewRows([]string{"reaction"}).AddRow("none"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpsertVideoCommentReactionSQL)).WithArgs("CMT_1", "user-1", "like").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateVideoCommentReactionShardSQL)).WithArgs("CMT_1", sqlmock.AnyArg(), int64(1), int64(0)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.MarkVideoCommentReactionDirtySQL)).WithArgs("CMT_1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionShardTotalsSQL)).WithArgs("CMT_1").WillReturnRows(sqlmock.NewRows([]string{"like_count", "dislike_count"}).AddRow(8, 2))
	mock.ExpectCommit()
	result, err := NewRepository(db).SetReaction(context.Background(), "user-1", "CMT_1", "like")
	if err != nil || result.LikeCount != 8 {
		t.Fatalf("SetReaction() result=%+v error=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCreateBindsOnlyOwnedPendingImagesInCommentTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByRequestIDSQL)).WithArgs("user-1", "user-1", "request-1").WillReturnRows(hgCommentRows(createdAt))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCommentableSubmissionSQL)).WithArgs("submission-1", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id"}).AddRow("submission-1"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentImageForAttachSQL)).WithArgs("https://cdn.example.com/video_comment/a.png", "https://cdn.example.com/video_comment/a.png", "user-1").WillReturnRows(sqlmock.NewRows([]string{"image_id"}).AddRow("CIMG_1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).WithArgs("CMT_1", "submission-1", "user-1", "request-1", nil, nil, nil, "hello", `["https://cdn.example.com/video_comment/a.png"]`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.AttachVideoCommentImageSQL)).WithArgs("CMT_1", "CIMG_1", "user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).WithArgs("user-1", "CMT_1").WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-1", "alice", "/a.png", "hello", nil, nil, nil, 0, 0, 0, "none", `["https://cdn.example.com/video_comment/a.png"]`, createdAt))
	mock.ExpectCommit()
	_, err = NewRepository(db).Create(context.Background(), HGCreateCommand{CommentID: "CMT_1", SubmissionID: "submission-1", UserID: "user-1", RequestID: "request-1", Content: "hello", ImageURLs: []string{"https://cdn.example.com/video_comment/a.png"}})
	if err != nil {
		t.Fatalf("Create() error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestDeleteDecrementsShardAndRootExactlyOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentDeleteTargetForUpdateSQL)).
		WithArgs("CMT_REPLY", "user-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id", "root_comment_id", "reply_count"}).AddRow("submission-1", "CMT_ROOT", 0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.SoftDeleteVideoCommentSQL)).
		WithArgs("CMT_REPLY", "user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DecrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DecrementVideoCommentReplyCountSQL)).
		WithArgs("CMT_ROOT").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.MarkVideoCommentImagesDeletePendingSQL)).
		WithArgs("CMT_REPLY").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	deleted, err := NewRepository(db).Delete(context.Background(), "user-1", "CMT_REPLY")
	if err != nil || !deleted {
		t.Fatalf("Delete() deleted=%v error=%v", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestDeleteRejectsRootCommentWithReplies(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentDeleteTargetForUpdateSQL)).
		WithArgs("CMT_ROOT", "user-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id", "root_comment_id", "reply_count"}).AddRow("submission-1", nil, 2))
	mock.ExpectRollback()

	deleted, err := NewRepository(db).Delete(context.Background(), "user-1", "CMT_ROOT")
	if deleted || !errors.Is(err, ErrCommentHasReplies) {
		t.Fatalf("Delete() deleted=%v error=%v, want ErrCommentHasReplies", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestDeleteReplyRejectsMissingRootCounterMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentDeleteTargetForUpdateSQL)).
		WithArgs("CMT_REPLY", "user-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id", "root_comment_id", "reply_count"}).AddRow("submission-1", "CMT_ROOT", 0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.SoftDeleteVideoCommentSQL)).
		WithArgs("CMT_REPLY", "user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DecrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DecrementVideoCommentReplyCountSQL)).
		WithArgs("CMT_ROOT").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	deleted, err := NewRepository(db).Delete(context.Background(), "user-1", "CMT_REPLY")
	if deleted || !errors.Is(err, ErrCounterConsistency) {
		t.Fatalf("Delete() deleted=%v error=%v, want ErrCounterConsistency", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func hgCommentRows(_ time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "comment_id", "submission_id", "user_id", "user_name", "avatar_url", "content", "root_comment_id", "parent_comment_id", "reply_to_user_id", "like_count", "dislike_count", "reply_count", "reaction", "image_urls", "created_at"})
}

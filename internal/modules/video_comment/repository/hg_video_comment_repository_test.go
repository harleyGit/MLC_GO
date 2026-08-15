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
		WithArgs("CMT_1", "submission-1", "user-1", "request-1", nil, nil, nil, nil, "hello", `[]`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("user-1", "CMT_1").WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-1", "alice", "/a.png", "hello", nil, nil, nil, "", 0, 0, 0, "none", `[]`, createdAt))
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
		WithArgs("CMT_1", "submission-1", "user-1", "request-1", nil, nil, nil, nil, "hello", `[]`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("user-1", "CMT_1").
		WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-1", "alice", "/a.png", "hello", nil, nil, nil, "", 0, 0, 0, "none", `[]`, createdAt))
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
		WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-2", "alice", "/a.png", "hello", nil, nil, nil, "", 0, 0, 0, "none", `[]`, createdAt))
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
		WithArgs("CMT_PARENT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id", "root_comment_id", "user_id", "user_name"}).AddRow("CMT_PARENT", "CMT_ROOT", "user-2", "bob"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentRootForUpdateSQL)).
		WithArgs("CMT_ROOT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id"}).AddRow("CMT_ROOT"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentImageForAttachSQL)).
		WithArgs("/uploads/video_comment/a.png", "/uploads/video_comment/a.png", "user-1").WillReturnRows(sqlmock.NewRows([]string{"image_id"}).AddRow("CIMG_1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_REPLY", "submission-1", "user-1", "request-1", "CMT_ROOT", "CMT_PARENT", "user-2", "bob", "hello", `["/uploads/video_comment/a.png"]`).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.AttachVideoCommentImageSQL)).
		WithArgs("CMT_REPLY", "CIMG_1", "user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	hgExpectReplyShardIncrement(mock, "CMT_ROOT", "CMT_REPLY")
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("user-1", "CMT_REPLY").WillReturnRows(hgCommentRows(createdAt).AddRow(2, "CMT_REPLY", "submission-1", "user-1", "alice", "/a.png", "hello", "CMT_ROOT", "CMT_PARENT", "user-2", "bob", 0, 0, 0, "none", `["/uploads/video_comment/a.png"]`, createdAt))
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
		WithArgs("CMT_PARENT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id", "root_comment_id", "user_id", "user_name"}).AddRow("CMT_PARENT", "CMT_ROOT", "user-2", "bob"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentRootForUpdateSQL)).
		WithArgs("CMT_ROOT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id"}).AddRow("CMT_ROOT"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_REPLY", "submission-1", "user-1", "request-1", "CMT_ROOT", "CMT_PARENT", "user-2", "bob", "hello", `[]`).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 2))
	hgExpectReplyShardIncrement(mock, "CMT_ROOT", "CMT_REPLY")
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("user-1", "CMT_REPLY").WillReturnRows(hgCommentRows(createdAt).AddRow(2, "CMT_REPLY", "submission-1", "user-1", "alice", "/a.png", "hello", "CMT_ROOT", "CMT_PARENT", "user-2", "bob", 0, 0, 0, "none", `[]`, createdAt))
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
		WithArgs("CMT_ROOT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id", "root_comment_id", "user_id", "user_name"}).AddRow("CMT_ROOT", nil, "user-2", "bob"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_REPLY", "submission-1", "user-1", "request-1", "CMT_ROOT", "CMT_ROOT", "user-2", "bob", "hello", `[]`).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	hgExpectReplyShardIncrement(mock, "CMT_ROOT", "CMT_REPLY")
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).
		WithArgs("user-1", "CMT_REPLY").WillReturnRows(hgCommentRows(createdAt).AddRow(2, "CMT_REPLY", "submission-1", "user-1", "alice", "/a.png", "hello", "CMT_ROOT", "CMT_ROOT", "user-2", "bob", 0, 0, 0, "none", `[]`, createdAt))
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
		WithArgs("CMT_1", "submission-1", "user-1", "request-1", nil, nil, nil, nil, "hello", `[]`).WillReturnResult(sqlmock.NewResult(1, 1))
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
		WithArgs("CMT_ROOT", "submission-1").WillReturnRows(sqlmock.NewRows([]string{"comment_id", "root_comment_id", "user_id", "user_name"}).AddRow("CMT_ROOT", nil, "user-2", "bob"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).
		WithArgs("CMT_REPLY", "submission-1", "user-1", "request-1", "CMT_ROOT", "CMT_ROOT", "user-2", "bob", "hello", `[]`).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).
		WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureVideoCommentReplyShardSQL)).
		WithArgs("CMT_ROOT").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentReplyShardSQL)).
		WithArgs("CMT_ROOT", hgReplyShard("CMT_REPLY")).WillReturnError(errors.New("reply shard unavailable"))
	mock.ExpectRollback()

	_, err = NewRepository(db).Create(context.Background(), HGCreateCommand{
		CommentID: "CMT_REPLY", SubmissionID: "submission-1", UserID: "user-1", RequestID: "request-1", ParentCommentID: "CMT_ROOT", Content: "hello",
	})
	if err == nil {
		t.Fatal("Create() expected reply shard error")
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
		WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-2", "alice", "/a.png", "hello", nil, nil, nil, "", 4, 1, 2, "dislike", `[]`, createdAt))
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
		WillReturnRows(hgCommentRows(createdAt).AddRow(2, "CMT_REPLY", "submission-1", "user-2", "alice", "/a.png", "reply", "CMT_ROOT", "CMT_ROOT", "user-3", "charlie", 0, 0, 0, "none", `[]`, createdAt))

	result, err := NewRepository(db).ListReplies(context.Background(), "user-1", "CMT_ROOT", HGListCursor{}, 21)
	if err != nil || result.TotalCount != 4 || len(result.Comments) != 1 {
		t.Fatalf("ListReplies() result=%+v error=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestListRepliesBatchHydratesLegacyReplyNamesOncePerPage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	createdAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentRootListMetadataSQL)).
		WithArgs("CMT_ROOT").WillReturnRows(sqlmock.NewRows([]string{"submission_id", "reply_count"}).AddRow("submission-1", 2))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListVideoCommentRepliesFirstSQL)).
		WithArgs("user-1", "submission-1", "CMT_ROOT", 21).
		WillReturnRows(hgCommentRows(createdAt).
			AddRow(2, "CMT_REPLY_1", "submission-1", "user-2", "alice", "/a.png", "reply", "CMT_ROOT", "CMT_ROOT", "user-3", "", 0, 0, 0, "none", `[]`, createdAt).
			AddRow(3, "CMT_REPLY_2", "submission-1", "user-4", "dave", "/d.png", "reply", "CMT_ROOT", "CMT_REPLY_1", "user-3", "", 0, 0, 0, "none", `[]`, createdAt.Add(time.Second)))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReplyUserNamesSQL)).
		WithArgs(`["user-3"]`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_name"}).AddRow("user-3", "charlie"))

	result, err := NewRepository(db).ListReplies(context.Background(), "user-1", "CMT_ROOT", HGListCursor{}, 21)
	if err != nil || len(result.Comments) != 2 || result.Comments[0].ReplyToUserName != "charlie" || result.Comments[1].ReplyToUserName != "charlie" {
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
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionBackfillReadySQL)).WillReturnRows(sqlmock.NewRows([]string{"completed"}).AddRow(true))
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

func TestProjectReplyCountsKeepsDirtyShardOnConcurrentWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListVideoCommentReplyDirtySQL)).
		WithArgs(10).WillReturnRows(sqlmock.NewRows([]string{"root_comment_id", "shard_id", "revision"}).AddRow("CMT_ROOT", 7, 3))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.ProjectVideoCommentReplyCountSQL)).
		WithArgs("CMT_ROOT").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DeleteVideoCommentReplyDirtySQL)).
		WithArgs("CMT_ROOT", uint32(7), uint64(3)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.RequeueVideoCommentReplyDirtySQL)).
		WithArgs("CMT_ROOT", uint32(7), uint64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := NewRepository(db).ProjectReplyCounts(context.Background(), 10)
	if err != nil || result.Projected != 1 || result.CASMisses != 1 {
		t.Fatalf("ProjectReplyCounts() result=%+v error=%v", result, err)
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
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionBackfillReadySQL)).WillReturnRows(sqlmock.NewRows([]string{"completed"}).AddRow(true))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionTargetSQL)).WithArgs("CMT_1").WillReturnRows(sqlmock.NewRows([]string{"comment_id"}).AddRow("CMT_1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureVideoCommentReactionSQL)).WithArgs("CMT_1", "user-1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionForUpdateSQL)).WithArgs("CMT_1", "user-1").WillReturnRows(sqlmock.NewRows([]string{"reaction"}).AddRow("none"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpsertVideoCommentReactionSQL)).WithArgs("CMT_1", "user-1", "like").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateVideoCommentReactionShardSQL)).WithArgs("CMT_1", sqlmock.AnyArg(), int64(1), int64(0), int64(1), int64(0)).WillReturnResult(sqlmock.NewResult(1, 1))
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

func TestSetReactionSwitchUsesNonNegativeInsertValuesAndSignedUpdateDeltas(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionBackfillReadySQL)).WillReturnRows(sqlmock.NewRows([]string{"completed"}).AddRow(true))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionTargetSQL)).WithArgs("CMT_1").WillReturnRows(sqlmock.NewRows([]string{"comment_id"}).AddRow("CMT_1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureVideoCommentReactionSQL)).WithArgs("CMT_1", "user-1").WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionForUpdateSQL)).WithArgs("CMT_1", "user-1").WillReturnRows(sqlmock.NewRows([]string{"reaction"}).AddRow("like"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpsertVideoCommentReactionSQL)).WithArgs("CMT_1", "user-1", "dislike").WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateVideoCommentReactionShardSQL)).WithArgs("CMT_1", sqlmock.AnyArg(), int64(0), int64(1), int64(-1), int64(1)).WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.MarkVideoCommentReactionDirtySQL)).WithArgs("CMT_1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionShardTotalsSQL)).WithArgs("CMT_1").WillReturnRows(sqlmock.NewRows([]string{"like_count", "dislike_count"}).AddRow(7, 3))
	mock.ExpectCommit()

	result, err := NewRepository(db).SetReaction(context.Background(), "user-1", "CMT_1", "dislike")
	if err != nil || result.LikeCount != 7 || result.DislikeCount != 3 || result.Reaction != "dislike" {
		t.Fatalf("SetReaction() result=%+v error=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestReserveImageAssetPersistsQuotaAndReservationInOneTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	asset := HGImageAsset{ImageID: "CIMG_1", UserID: "user-1", StorageKey: "video_comment/a.png", ImageURL: "https://cdn.example.com/video_comment/a.png", SizeBytes: 3, ContentType: "image/png"}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureVideoCommentImageQuotaSQL)).WithArgs("user-1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.ReserveVideoCommentImageQuotaSQL)).WithArgs(int64(3), "user-1", int64(3), int64(100)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentImageAssetSQL)).WithArgs("CIMG_1", "user-1", "video_comment/a.png", "https://cdn.example.com/video_comment/a.png", int64(3), "image/png").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := NewRepository(db).ReserveImageAsset(context.Background(), asset, 100); err != nil {
		t.Fatalf("ReserveImageAsset() error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestProjectReactionCountsDeletesOnlySelectedRevision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListVideoCommentReactionDirtySQL)).WithArgs(10).WillReturnRows(sqlmock.NewRows([]string{"comment_id", "revision"}).AddRow("CMT_1", 7))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.ProjectVideoCommentReactionCountsSQL)).WithArgs("CMT_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DeleteVideoCommentReactionDirtySQL)).WithArgs("CMT_1", uint64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.RequeueVideoCommentReactionDirtySQL)).WithArgs("CMT_1", uint64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := NewRepository(db).ProjectReactionCounts(context.Background(), 10)
	if err != nil || result.Projected != 1 || result.CASMisses != 1 {
		t.Fatalf("ProjectReactionCounts() result=%+v error=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSetReactionFailsClosedWhileHistoricalBackfillIsIncomplete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionBackfillReadySQL)).WillReturnRows(sqlmock.NewRows([]string{"completed"}).AddRow(false))
	mock.ExpectRollback()

	_, err = NewRepository(db).SetReaction(context.Background(), "user-1", "CMT_1", "like")
	if !errors.Is(err, ErrReactionBackfillIncomplete) {
		t.Fatalf("SetReaction() error=%v, want ErrReactionBackfillIncomplete", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCompleteImageCleanupRequiresCurrentFencingToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	asset := HGImageCleanupAsset{ImageID: "CIMG_1", UserID: "user-1", StorageKey: "video_comment/a.png", SizeBytes: 3, CleanupToken: "token-1"}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DeleteVideoCommentImageAssetSQL)).WithArgs("CIMG_1", "token-1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := NewRepository(db).CompleteImageCleanup(context.Background(), asset); err != nil {
		t.Fatalf("CompleteImageCleanup() error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestReleaseImageCleanupNeverMakesClaimedObjectAttachableAgain(t *testing.T) {
	if strings.Contains(SQLQueriesPackage.ReleaseVideoCommentImageCleanupSQL, "'pending'") {
		t.Fatalf("ReleaseVideoCommentImageCleanupSQL must not restore pending: %s", SQLQueriesPackage.ReleaseVideoCommentImageCleanupSQL)
	}
}

func TestImageCleanupQueriesUseOneIndexedStateRangeEach(t *testing.T) {
	queries := []string{
		SQLQueriesPackage.ListPendingVideoCommentImageCleanupForUpdateSQL,
		SQLQueriesPackage.ListDeletePendingVideoCommentImageCleanupForUpdateSQL,
		SQLQueriesPackage.ListExpiredVideoCommentImageCleanupForUpdateSQL,
	}
	for _, query := range queries {
		if strings.Contains(strings.ToUpper(query), " OR ") || !strings.Contains(query, "FOR UPDATE SKIP LOCKED") {
			t.Fatalf("cleanup query must use one indexed state range: %s", query)
		}
	}
}

func TestMaintenanceOldestQueriesUseIndexedOrderAndLimitOne(t *testing.T) {
	queries := []string{
		SQLQueriesPackage.SelectVideoCommentReactionDirtyOldestSQL,
		SQLQueriesPackage.SelectPendingVideoCommentImageCleanupOldestSQL,
		SQLQueriesPackage.SelectDeletePendingVideoCommentImageCleanupOldestSQL,
		SQLQueriesPackage.SelectExpiredVideoCommentImageCleanupOldestSQL,
	}
	for _, query := range queries {
		upper := strings.ToUpper(query)
		if strings.Contains(upper, " OR ") || !strings.Contains(upper, "ORDER BY") || !strings.Contains(upper, "LIMIT 1") {
			t.Fatalf("oldest metric query must use one indexed ordered range: %s", query)
		}
	}
}

func TestMaintenanceOldestTimesReturnsOldestEligibleCleanupState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	orphanBefore := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	dirtyOldest := orphanBefore.Add(-time.Hour)
	pendingOldest := orphanBefore.Add(-2 * time.Hour)
	deleteOldest := orphanBefore.Add(-3 * time.Hour)
	leaseOldest := orphanBefore.Add(-30 * time.Minute)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionDirtyOldestSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(dirtyOldest))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectDeletePendingVideoCommentImageCleanupOldestSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"delete_after"}).AddRow(deleteOldest))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectExpiredVideoCommentImageCleanupOldestSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"cleanup_lease_until"}).AddRow(leaseOldest))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectPendingVideoCommentImageCleanupOldestSQL)).WithArgs(orphanBefore).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(pendingOldest))

	dirty, cleanup, err := NewRepository(db).MaintenanceOldestTimes(context.Background(), orphanBefore, time.Hour)
	if err != nil || !dirty.Equal(dirtyOldest) || !cleanup.Equal(deleteOldest) {
		t.Fatalf("MaintenanceOldestTimes() dirty=%v cleanup=%v error=%v", dirty, cleanup, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestMaintenanceOldestTimesMeasuresPendingAgeSinceEligibility(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	orphanAge := 24 * time.Hour
	orphanBefore := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	createdAt := orphanBefore.Add(-5 * time.Minute)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentReactionDirtyOldestSQL)).WillReturnRows(sqlmock.NewRows([]string{"updated_at"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectDeletePendingVideoCommentImageCleanupOldestSQL)).WillReturnRows(sqlmock.NewRows([]string{"delete_after"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectExpiredVideoCommentImageCleanupOldestSQL)).WillReturnRows(sqlmock.NewRows([]string{"cleanup_lease_until"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectPendingVideoCommentImageCleanupOldestSQL)).WithArgs(orphanBefore).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(createdAt))

	_, cleanup, err := NewRepository(db).MaintenanceOldestTimes(context.Background(), orphanBefore, orphanAge)
	if err != nil || !cleanup.Equal(createdAt.Add(orphanAge)) {
		t.Fatalf("MaintenanceOldestTimes() cleanup=%v error=%v", cleanup, err)
	}
}

func TestClaimImageCleanupCountsOnlySuccessfullyReclaimedExpiredLeases(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	orphanBefore := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListDeletePendingVideoCommentImageCleanupForUpdateSQL)).WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"image_id", "user_id", "storage_key", "size_bytes"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListExpiredVideoCommentImageCleanupForUpdateSQL)).WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"image_id", "user_id", "storage_key", "size_bytes"}).
			AddRow("CIMG_1", "user-1", "video_comment/a.png", 3).
			AddRow("CIMG_2", "user-2", "video_comment/b.png", 4))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.MarkVideoCommentImageDeletingSQL)).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "CIMG_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.MarkVideoCommentImageDeletingSQL)).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "CIMG_2").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	claim, err := NewRepository(db).ClaimImageCleanup(context.Background(), orphanBefore, 2, time.Minute)
	if err != nil || len(claim.Assets) != 1 || claim.ExpiredLeaseReclaims != 1 {
		t.Fatalf("ClaimImageCleanup() claim=%+v error=%v", claim, err)
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
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoCommentSQL)).WithArgs("CMT_1", "submission-1", "user-1", "request-1", nil, nil, nil, nil, "hello", `["https://cdn.example.com/video_comment/a.png"]`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.AttachVideoCommentImageSQL)).WithArgs("CMT_1", "CIMG_1", "user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentStatShardSQL)).WithArgs("submission-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCommentByCommentIDSQL)).WithArgs("user-1", "CMT_1").WillReturnRows(hgCommentRows(createdAt).AddRow(1, "CMT_1", "submission-1", "user-1", "alice", "/a.png", "hello", nil, nil, nil, "", 0, 0, 0, "none", `["https://cdn.example.com/video_comment/a.png"]`, createdAt))
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
	hgExpectReplyShardDecrement(mock, "CMT_ROOT", "CMT_REPLY", 1)
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
	hgExpectReplyShardDecrement(mock, "CMT_ROOT", "CMT_REPLY", 0)
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
	return sqlmock.NewRows([]string{"id", "comment_id", "submission_id", "user_id", "user_name", "avatar_url", "content", "root_comment_id", "parent_comment_id", "reply_to_user_id", "reply_to_user_name", "like_count", "dislike_count", "reply_count", "reaction", "image_urls", "created_at"})
}

func hgExpectReplyShardIncrement(mock sqlmock.Sqlmock, rootCommentID, commentID string) {
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureVideoCommentReplyShardSQL)).
		WithArgs(rootCommentID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoCommentReplyShardSQL)).
		WithArgs(rootCommentID, hgReplyShard(commentID)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.MarkVideoCommentReplyDirtySQL)).
		WithArgs(rootCommentID, hgReplyShard(commentID)).WillReturnResult(sqlmock.NewResult(0, 1))
}

func hgExpectReplyShardDecrement(mock sqlmock.Sqlmock, rootCommentID, commentID string, affected int64) {
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureVideoCommentReplyShardSQL)).
		WithArgs(rootCommentID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DecrementVideoCommentReplyShardSQL)).
		WithArgs(rootCommentID, hgReplyShard(commentID)).WillReturnResult(sqlmock.NewResult(0, affected))
	if affected == 1 {
		mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.MarkVideoCommentReplyDirtySQL)).
			WithArgs(rootCommentID, hgReplyShard(commentID)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
}

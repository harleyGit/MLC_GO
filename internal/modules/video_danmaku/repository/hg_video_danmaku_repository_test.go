package VideoDanmakuRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDanmakuVideoTargetMatchesVisibleVideoStatuses(t *testing.T) {
	query := SQLQueriesPackage.SelectDanmakuVideoTargetSQL
	if !strings.Contains(query, "status IN ('reviewing', 'published')") {
		t.Fatalf("SelectDanmakuVideoTargetSQL = %q, want reviewing and published statuses", query)
	}
	if !strings.Contains(query, "visibility = 'public'") || !strings.Contains(query, "close_danmaku = 0") {
		t.Fatalf("SelectDanmakuVideoTargetSQL = %q, want public and danmaku-enabled guards", query)
	}
}

func TestCreateReadsInsertedDanmakuByPrimaryKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT vf\\.video_id, vs\\.submission_id").
		WithArgs("video-1").
		WillReturnRows(sqlmock.NewRows([]string{"video_id", "submission_id"}).AddRow("video-1", "submission-1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertVideoDanmakuSQL)).
		WithArgs("DMK_1", "submission-1", "video-1", "user-1", "request-1", uint32(1000), "hello", "scroll", "#FFFFFF", uint8(25)).
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.IncrementVideoDanmakuStatShardSQL)).
		WithArgs("video-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoDanmakuByPrimaryIDSQL)).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "danmaku_id", "submission_id", "video_id", "user_id", "progress_ms", "content", "mode", "color", "font_size", "created_at"}).
			AddRow(42, "DMK_1", "submission-1", "video-1", "user-1", 1000, "hello", "scroll", "#FFFFFF", 25, time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)))
	mock.ExpectCommit()

	item, created, err := NewRepository(db).Create(context.Background(), HGCreateCommand{DanmakuID: "DMK_1", VideoID: "video-1", UserID: "user-1", RequestID: "request-1", Content: "hello", ProgressMS: 1000, Mode: "scroll", Color: "#FFFFFF", FontSize: 25})
	if err != nil || !created || item.ID != 42 {
		t.Fatalf("Create() item=%+v created=%t error=%v", item, created, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestResolveVideoAcceptsVisibleReviewingTarget(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT vf\\.video_id, vs\\.submission_id").
		WithArgs("video-1").
		WillReturnRows(sqlmock.NewRows([]string{"video_id", "submission_id"}).AddRow("video-1", "submission-1"))

	submissionID, err := NewRepository(db).ResolveVideo(context.Background(), "video-1")
	if err != nil || submissionID != "submission-1" {
		t.Fatalf("ResolveVideo() submissionID=%q error=%v", submissionID, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

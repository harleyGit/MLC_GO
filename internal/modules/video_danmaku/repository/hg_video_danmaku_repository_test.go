package VideoDanmakuRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"strings"
	"testing"

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

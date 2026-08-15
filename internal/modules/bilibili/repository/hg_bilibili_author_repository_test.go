package BilibiliRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAuthorVideoQueriesIncludeCurrentPublicSubmissionStates(t *testing.T) {
	for name, query := range map[string]string{
		"first page": SQLQueriesPackage.SelectBilibiliAuthorVideosFirstSQL,
		"next page":  SQLQueriesPackage.SelectBilibiliAuthorVideosByCursorSQL,
		"count":      SQLQueriesPackage.CountBilibiliAuthorVideosSQL,
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(query, "status IN ('reviewing', 'published')") {
				t.Fatalf("query does not include current public submission states: %s", query)
			}
		})
	}
	if !strings.Contains(SQLQueriesPackage.SelectBilibiliAuthorVideosFirstSQL, "COALESCE(vs.publish_time, vs.submit_time, vs.created_at)") {
		t.Fatalf("first page query does not provide a historical publish-time fallback")
	}
}

func TestGetVideosReturnsReviewingSubmissionWithFallbackPublishTime(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	publishTime := time.Date(2026, time.August, 15, 8, 30, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectBilibiliAuthorVideosFirstSQL)).
		WithArgs("user-1", 21).
		WillReturnRows(sqlmock.NewRows([]string{
			"submission_id", "video_id", "user_id", "title", "cover_url", "category",
			"description", "duration", "file_path", "profile_publish_time",
		}).AddRow("submission-1", "video-1", "user-1", "测试视频", "cover.jpg", "动画", "简介", 90, "video.mp4", publishTime))

	videos, err := NewRepository(db).GetVideos(context.Background(), "user-1", nil, "", 21)
	if err != nil {
		t.Fatalf("GetVideos() error = %v", err)
	}
	if len(videos) != 1 || videos[0].SubmissionID != "submission-1" {
		t.Fatalf("videos = %#v, want submission-1", videos)
	}
	if videos[0].PublishTime != publishTime.Format(time.RFC3339Nano) {
		t.Fatalf("PublishTime = %q, want %q", videos[0].PublishTime, publishTime.Format(time.RFC3339Nano))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

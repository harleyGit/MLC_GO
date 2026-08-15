package VideoRecommendRepositoryPackage

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRepositoryBatchGetPublicVideoCards(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	published := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta("WHERE vs.status = 'published' AND vs.visibility = 'public' AND vs.submission_id IN (?,?)")).
		WithArgs("s1", "s2").
		WillReturnRows(sqlmock.NewRows([]string{"submission_id", "video_id", "user_id", "title", "cover_url", "category", "description", "duration", "file_path", "publish_time"}).
			AddRow("s1", "v1", "u1", "title", "cover", "tech", "desc", 60, "path", published))
	items, err := NewRepository(db).BatchGetPublicVideoCards(context.Background(), []string{"s1", "s2"})
	if err != nil {
		t.Fatalf("BatchGetPublicVideoCards() error = %v", err)
	}
	if len(items) != 1 || items["s1"].VideoID != "v1" {
		t.Fatalf("items = %#v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

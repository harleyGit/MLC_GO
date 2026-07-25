package VideoUploadRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetVideoListByCursorUsesCursorArgsBeforeLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	submitTime := "2026-07-04T10:00:00Z"
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.GetVideoListByCursorSQL)).
		WithArgs(submitTime, submitTime, "submission_9", 21).
		WillReturnRows(sqlmock.NewRows([]string{
			"submission_id", "user_id", "title", "cover_url", "category", "video_type", "description", "visibility", "status",
			"video_count", "total_size", "submit_time", "created_at", "video_id", "file_path", "file_name", "file_size", "mime_type", "part_number",
		}))

	repo := NewRepository(db)
	if _, err := repo.GetVideoListByCursor(context.Background(), submitTime+"|submission_9", 21, ""); err != nil {
		t.Fatalf("GetVideoListByCursor() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetVideoListByCursorFiltersByTagName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.GetVideoListByTagCursorFirstSQL)).
		WithArgs("MMD·3D", 21).
		WillReturnRows(sqlmock.NewRows([]string{
			"submission_id", "user_id", "title", "cover_url", "category", "video_type", "description", "visibility", "status",
			"video_count", "total_size", "submit_time", "created_at", "video_id", "file_path", "file_name", "file_size", "mime_type", "part_number",
		}))

	repo := NewRepository(db)
	if _, err := repo.GetVideoListByCursor(context.Background(), "", 21, "MMD·3D"); err != nil {
		t.Fatalf("GetVideoListByCursor() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

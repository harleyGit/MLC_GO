package VideoCommentRepositoryPackage

import (
	hgQueries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
)

// TestHGExternalCommentRequiresStoredDetail 外部视频只有列表摘要时不能写评论。
func TestHGExternalCommentRequiresStoredDetail(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgID := "external:bilibili:BV1wQJ26XEPT"
	hgMock.ExpectBegin()
	hgMock.ExpectQuery(regexp.QuoteMeta(hgQueries.HGExternalCommentTargetSQL)).WithArgs(hgID, hgID).WillReturnError(sql.ErrNoRows)
	hgMock.ExpectRollback()
	_, hgErr = NewRepository(hgDB).Create(context.Background(), HGCreateCommand{SubmissionID: hgID})
	if !errors.Is(hgErr, ErrSubmissionNotCommentable) {
		t.Fatalf("缺失快照未拒绝: %v", hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

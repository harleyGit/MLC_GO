package VideoDanmakuRepositoryPackage

import (
	hgQueries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
)

// TestHGExternalCreateRejectsUnknownPart 伪造CID在事务写入前拒绝，不能产生计数或Outbox。
func TestHGExternalCreateRejectsUnknownPart(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgID := "external:bilibili:BV1wQJ26XEPT:0"
	hgMock.ExpectBegin()
	hgMock.ExpectQuery(regexp.QuoteMeta(hgQueries.HGExternalDanmakuTargetSQL)).WithArgs(hgID, hgID).WillReturnError(sql.ErrNoRows)
	hgMock.ExpectRollback()
	_, hgCreated, hgErr := NewRepository(hgDB).Create(context.Background(), HGCreateCommand{VideoID: hgID})
	if hgCreated || !errors.Is(hgErr, ErrVideoNotDanmakuEnabled) {
		t.Fatalf("无效分P未拒绝: %v", hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

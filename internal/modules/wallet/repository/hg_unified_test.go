package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"MLC_GO/internal/modules/wallet/model"
	hgqueries "MLC_GO/internal/pkg/mysql/queries"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGUnifiedRejectInconsistentTradeAndHash(t *testing.T) {
	for _, hgCase := range []struct {
		hgHash       string // 持久化请求哈希。
		hgConsistent bool   // SQL跨表一致性检查结果。
	}{{hgRequestHash("sku"), false}, {hgRequestHash("other"), true}} {
		hgDB, hgMock, hgErr := sqlmock.New()
		if hgErr != nil {
			t.Fatal(hgErr)
		}
		hgRows := sqlmock.NewRows([]string{"id", "user", "request", "sku", "name", "title", "description", "currency", "pay", "total", "created", "expires", "mode", "status", "paid", "transaction", "balance", "hash", "consistent"}).AddRow("order", "owner", "request", "sku", "name", "title", "description", "CNY", 600, 50, time.Now(), time.Now(), "platform_debug", "paid", time.Now(), 9, 70, hgCase.hgHash, hgCase.hgConsistent)
		hgMock.ExpectBegin()
		hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderForPaySQL)).WithArgs("order", "owner").WillReturnRows(hgRows)
		hgMock.ExpectRollback()
		if _, hgErr = HGNewRepository(hgDB).HGPayDebug(context.Background(), "owner", "order"); !errors.Is(hgErr, model.HGErrConflict) {
			t.Fatal(hgErr)
		}
		if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
			t.Fatal(hgErr)
		}
		hgDB.Close()
	}
}

func TestHGLegacyMigrationBoundedAndReplay(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgRepo := HGNewRepository(hgDB)
	if _, hgErr = hgRepo.HGMigrateLegacyRange(context.Background(), 1, 501); !errors.Is(hgErr, model.HGErrInvalid) {
		t.Fatal(hgErr)
	}
	hgNow := time.Now().UTC().Truncate(time.Millisecond)
	for _, hgReplay := range []bool{false, true} {
		hgMock.ExpectBegin()
		hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletLegacyRangeSQL)).WithArgs(uint64(1), uint64(500)).WillReturnRows(hgOrderRows(hgNow)).RowsWillBeClosed()
		hgRead := hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletLegacyByOwnerSQL)).WithArgs("order", "owner")
		if hgReplay {
			hgRead.WillReturnRows(hgOrderRows(hgNow))
		} else {
			hgRead.WillReturnError(sql.ErrNoRows)
			hgMock.ExpectExec(regexp.QuoteMeta(hgqueries.HGWalletInsertTradeSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
			hgMock.ExpectExec(regexp.QuoteMeta(hgqueries.HGWalletInsertLegacyOrderSQL)).WithArgs("order", "order", "owner", "request", hgRequestHash("sku"), "sku", "昵称", "标题", "说明", "CNY", uint64(600), uint64(50), hgNow, hgNow.Add(10*time.Minute), "unavailable", 1, nil, nil, nil, nil).WillReturnResult(sqlmock.NewResult(1, 1))
		}
		hgMock.ExpectCommit()
		if hgCount, hgErr := hgRepo.HGMigrateLegacyRange(context.Background(), 1, 500); hgErr != nil || hgCount != 1 {
			t.Fatal(hgCount, hgErr)
		}
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

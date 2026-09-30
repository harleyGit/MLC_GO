package accountrepository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"MLC_GO/internal/account"
	accountservice "MLC_GO/internal/account/service"
	hgqueries "MLC_GO/internal/pkg/mysql/queries"
	"github.com/DATA-DOG/go-sqlmock"
)

var _ account.HGAccountReader = (*HGRepository)(nil)

func TestHGAccountReadThroughService(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgNow := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGSelectAccountSQL)).WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "balance", "frozen_balance", "currency", "account_type", "version", "created_at", "updated_at"}).
			AddRow("user-1", 12, 0, "MLC_COIN", "user", 4, hgNow, hgNow))
	hgService := accountservice.NewHGService(NewHGRepository(hgDB))
	hgAccount, hgErr := hgService.GetAccount(context.Background(), "user-1")
	if hgErr != nil || hgAccount.UserID != "user-1" || hgAccount.Balance != 12 || hgAccount.FrozenBalance != 0 || hgAccount.Currency != "MLC_COIN" || hgAccount.AccountType != "user" || hgAccount.Version != 4 || !hgAccount.CreatedAt.Equal(hgNow) || !hgAccount.UpdatedAt.Equal(hgNow) {
		t.Fatalf("读取账户结果不符: %+v, %v", hgAccount, hgErr)
	}
	hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGSelectAccountSQL)).WithArgs("missing").WillReturnError(sql.ErrNoRows)
	if _, hgErr = hgService.GetAccount(context.Background(), "missing"); !errors.Is(hgErr, sql.ErrNoRows) {
		t.Fatalf("缺失账户必须返回错误: %v", hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

func TestHGLedgerReadPreservesHistoricalDecimalAndReference(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgNow := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	hgColumns := []string{"id", "user_id", "request_id", "operation", "amount", "signed_delta", "balance_after", "balance_before", "reason", "business_type", "business_key", "reference_transaction_id", "created_at"}
	hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGSelectAccountLedgerSQL)).WithArgs("user-1", 20).
		WillReturnRows(sqlmock.NewRows(hgColumns).
			AddRow(8, "user-1", "refund-1", "refund", 2, 2, 7, "5", "refund", "video_coin", "video-1", 4, hgNow).
			AddRow(7, "user-1", "legacy", "grant", 9, 9, 0, "-9", "legacy", "", "", nil, hgNow))
	hgItems, hgErr := accountservice.NewHGService(NewHGRepository(hgDB)).ListLedger(context.Background(), "user-1", 101)
	if hgErr != nil || len(hgItems) != 2 {
		t.Fatalf("流水读取失败: %+v, %v", hgItems, hgErr)
	}
	if hgItems[0].ID != 8 || hgItems[0].ReferenceTransactionID != 4 || hgItems[0].BalanceBefore != "5" || hgItems[1].ReferenceTransactionID != 0 || hgItems[1].BalanceBefore != "-9" {
		t.Fatalf("流水关联或历史金额丢失: %+v", hgItems)
	}
	hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGSelectAccountLedgerSQL)).WithArgs("user-1", 1).
		WillReturnRows(sqlmock.NewRows(hgColumns).AddRow(8, "user-1", "refund-1", "refund", 2, 2, 7, "5", "", "", "", nil, hgNow).RowError(0, sql.ErrConnDone))
	if _, hgErr = NewHGRepository(hgDB).ListLedger(context.Background(), "user-1", 1); !errors.Is(hgErr, sql.ErrConnDone) {
		t.Fatalf("遍历失败未上报: %v", hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

func TestHGServiceRejectsMissingReader(t *testing.T) {
	hgService := accountservice.NewHGService(nil)
	if _, hgErr := hgService.GetAccount(context.Background(), "user-1"); hgErr == nil {
		t.Fatal("缺失仓储不能返回成功")
	}
	if _, hgErr := hgService.ListLedger(context.Background(), "user-1", 1); hgErr == nil {
		t.Fatal("缺失仓储不能返回成功")
	}
}

func TestHGRepositoryRejectsNilDatabase(t *testing.T) {
	hgRepository := NewHGRepository(nil)
	if _, hgErr := hgRepository.GetAccount(context.Background(), "user-1"); hgErr == nil {
		t.Fatal("空数据库不能读取账户")
	}
	if _, hgErr := hgRepository.ListLedger(context.Background(), "user-1", 100); hgErr == nil {
		t.Fatal("空数据库不能读取账户流水")
	}
}

func TestHGLedgerLimitBoundaries(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	// 独立约束真实 SQL 的过滤、稳定排序及 LIMIT，不用常量自身充当期望。
	hgPattern := `FROM account_ledger FORCE INDEX \(idx_coin_transaction_user_created\) WHERE user_id = \? ORDER BY created_at DESC, id DESC LIMIT \?$`
	for _, hgCase := range []struct {
		hgInput int // hgInput 是调用方请求页大小。
		hgWant  int // hgWant 是实际绑定上限，非法输入回落为 20。
	}{{-1, 20}, {0, 20}, {1, 1}, {100, 100}, {101, 20}} {
		hgMock.ExpectQuery(hgPattern).WithArgs("user-1", hgCase.hgWant).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))
		if _, hgErr = NewHGRepository(hgDB).ListLedger(context.Background(), "user-1", hgCase.hgInput); hgErr != nil {
			t.Fatal(hgErr)
		}
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

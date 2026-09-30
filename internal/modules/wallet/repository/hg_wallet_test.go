package repository

import (
	"MLC_GO/internal/modules/wallet/model"
	hgqueries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func hgOrderRows(hgNow time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"order_id", "user_id", "request_id", "sku_id", "display_name", "title", "description", "currency", "pay_amount", "total_coin", "created_at", "expires_at"}).AddRow("order", "owner", "request", "sku", "昵称", "标题", "说明", "CNY", 600, 50, hgNow, hgNow.Add(10*time.Minute))
}
func hgSKURows(hgNow time.Time, hgCoins uint64) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "sku_id", "title", "currency", "pay_amount", "coin_amount", "bonus_coin", "total_coin", "start_time", "end_time"}).AddRow(1, "sku", "标题", "CNY", 600, hgCoins, 0, hgCoins, hgNow, nil)
}

func TestHGWalletCreateSnapshotAndDuplicate(t *testing.T) {
	for _, hgDuplicate := range []bool{false, true} {
		t.Run(map[bool]string{false: "创建", true: "并发重复键恢复"}[hgDuplicate], func(t *testing.T) {
			hgDB, hgMock, hgErr := sqlmock.New()
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgDB.Close()
			hgNow := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
			hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderByRequestSQL)).WithArgs("owner", "request").WillReturnError(sql.ErrNoRows)
			hgMock.ExpectBegin()
			hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletSKUForShareSQL)).WithArgs("sku", hgNow, hgNow).WillReturnRows(hgSKURows(hgNow, 50))
			hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletDisplayNameSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("昵称"))
			hgInsert := hgMock.ExpectExec(regexp.QuoteMeta(hgqueries.HGWalletInsertOrderSQL)).WithArgs(sqlmock.AnyArg(), "owner", "request", "sku", "昵称", "标题", "充值获得50平台币", "CNY", uint64(600), uint64(50), hgNow, hgNow.Add(10*time.Minute))
			if hgDuplicate {
				hgInsert.WillReturnError(&mysql.MySQLError{Number: 1062})
				hgMock.ExpectRollback()
				hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderByRequestSQL)).WithArgs("owner", "request").WillReturnRows(hgOrderRows(hgNow))
			} else {
				hgInsert.WillReturnResult(sqlmock.NewResult(1, 1))
				hgMock.ExpectCommit()
			}
			hgOrder, hgErr := HGNewRepository(hgDB).HGCreate(context.Background(), "owner", "sku", "request", hgNow)
			if hgErr != nil || hgOrder.HGPayAmount != 600 || hgOrder.HGTotalCoin != 50 || hgOrder.HGDisplayName != "昵称" || !hgOrder.HGExpiresAt.Equal(hgNow.Add(10*time.Minute)) {
				t.Fatal(hgOrder, hgErr)
			}
			if !hgDuplicate && len(hgOrder.HGOrderID) != 64 {
				t.Fatal("订单随机ID长度")
			}
			if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

func TestHGWalletReplayAndOwner(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgRepo := HGNewRepository(hgDB)
	hgNow := time.Now().UTC()
	for _, hgSKU := range []string{"sku", "other"} {
		hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderByRequestSQL)).WithArgs("owner", "request").WillReturnRows(hgOrderRows(hgNow))
		hgOrder, hgErr := hgRepo.HGCreate(context.Background(), "owner", hgSKU, "request", hgNow.Add(time.Hour))
		if hgSKU == "sku" && (hgErr != nil || !hgOrder.HGExpiresAt.Equal(hgNow.Add(10*time.Minute))) {
			t.Fatal(hgOrder, hgErr)
		}
		if hgSKU == "other" && !errors.Is(hgErr, model.HGErrConflict) {
			t.Fatal(hgErr)
		}
	}
	hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderByOwnerSQL)).WithArgs("order", "other").WillReturnError(sql.ErrNoRows)
	if _, hgErr := hgRepo.HGDetail(context.Background(), "other", "order"); !errors.Is(hgErr, model.HGErrNotFound) {
		t.Fatal(hgErr)
	}
	if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

func TestHGWalletRejectSKUAndRollback(t *testing.T) {
	for _, hgCase := range []string{"unavailable", "too-large", "database"} {
		t.Run(hgCase, func(t *testing.T) {
			hgDB, hgMock, hgErr := sqlmock.New()
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgDB.Close()
			hgNow := time.Now()
			hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderByRequestSQL)).WithArgs("owner", "request").WillReturnError(sql.ErrNoRows)
			hgMock.ExpectBegin()
			hgSelect := hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletSKUForShareSQL)).WithArgs("sku", hgNow, hgNow)
			hgWant := model.HGErrUnsupported
			switch hgCase {
			case "unavailable":
				hgSelect.WillReturnError(sql.ErrNoRows)
				hgWant = model.HGErrSKUUnavailable
			case "too-large":
				hgSelect.WillReturnRows(hgSKURows(hgNow, 1001))
			default:
				hgWant = context.DeadlineExceeded
				hgSelect.WillReturnError(hgWant)
			}
			hgMock.ExpectRollback()
			if hgCase == "unavailable" {
				hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderByRequestSQL)).WithArgs("owner", "request").WillReturnError(sql.ErrNoRows)
			}
			if _, hgErr := HGNewRepository(hgDB).HGCreate(context.Background(), "owner", "sku", "request", hgNow); !errors.Is(hgErr, hgWant) {
				t.Fatal(hgErr)
			}
			if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

func TestHGWalletListWindow(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletSKUWindowSQL)).WithArgs(uint64(7)).WillReturnRows(hgSKURows(time.Now(), 50)).RowsWillBeClosed()
	hgItems, hgErr := HGNewRepository(hgDB).HGListCandidates(context.Background(), 7)
	if hgErr != nil || len(hgItems) != 1 || hgItems[0].HGEndTime != nil {
		t.Fatal(hgItems, hgErr)
	}
	if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

// TestHGWalletAmountSafety 验证金额安全整数上界和币数组合校验，非法档位不得写订单。
func TestHGWalletAmountSafety(t *testing.T) {
	for _, hgCase := range []struct {
		hgName                          string
		hgPay, hgCoin, hgBonus, hgTotal uint64
		hgValid                         bool
	}{
		{"安全上界", 9007199254740991, 40, 10, 50, true},
		{"超过JS安全上界", 9007199254740992, 40, 10, 50, false},
		{"零金额", 0, 40, 10, 50, false},
		{"币数不一致", 600, 40, 11, 50, false},
		{"防减法下溢", 600, 51, 0, 50, false},
		{"零基础币", 600, 0, 50, 50, false},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgDB, hgMock, hgErr := sqlmock.New()
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgDB.Close()
			hgNow := time.Now().UTC().Truncate(time.Millisecond)
			hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderByRequestSQL)).WithArgs("owner", "request").WillReturnError(sql.ErrNoRows)
			hgMock.ExpectBegin()
			hgRows := sqlmock.NewRows([]string{"id", "sku_id", "title", "currency", "pay_amount", "coin_amount", "bonus_coin", "total_coin", "start_time", "end_time"}).AddRow(1, "sku", "标题", "CNY", hgCase.hgPay, hgCase.hgCoin, hgCase.hgBonus, hgCase.hgTotal, hgNow, nil)
			hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletSKUForShareSQL)).WithArgs("sku", hgNow, hgNow).WillReturnRows(hgRows)
			if hgCase.hgValid {
				hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletDisplayNameSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("昵称"))
				hgMock.ExpectExec(regexp.QuoteMeta(hgqueries.HGWalletInsertOrderSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
				hgMock.ExpectCommit()
			} else {
				hgMock.ExpectRollback()
			}
			hgOrder, hgErr := HGNewRepository(hgDB).HGCreate(context.Background(), "owner", "sku", "request", hgNow)
			if hgCase.hgValid && (hgErr != nil || hgOrder.HGPayAmount != hgCase.hgPay) {
				t.Fatal(hgOrder, hgErr)
			}
			if !hgCase.hgValid && !errors.Is(hgErr, model.HGErrSKUUnavailable) {
				t.Fatal(hgErr)
			}
			if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

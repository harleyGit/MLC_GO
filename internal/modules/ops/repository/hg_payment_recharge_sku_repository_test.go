package OpsRepositoryPackage

import (
	OpsModelPackage "MLC_GO/internal/modules/ops/model"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func TestHGRechargeSKUOptimisticMutations(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		for _, affected := range []int64{0, 1} {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repo := NewRepository(db)
			item := OpsModelPackage.HGPaymentRechargeSKU{SKUID: "RSKU_test", SKUCode: "six", Title: "six", PayAmount: 600, CoinAmount: 600, TotalCoin: 600, Status: 1, StartTime: time.Now(), Version: 1}
			if deleting {
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DeleteOpsPaymentRechargeSKUSQL)).WithArgs("admin", item.SKUID, item.Version).WillReturnResult(sqlmock.NewResult(0, affected))
				err = repo.HGDeleteRechargeSKU(context.Background(), "admin", item.SKUID, item.Version)
			} else {
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateOpsPaymentRechargeSKUSQL)).WithArgs(item.SKUCode, item.Title, item.PayAmount, item.CoinAmount, item.BonusCoin, item.TotalCoin, item.Status, item.SortOrder, item.StartTime, nil, "admin", item.SKUID, item.Version).WillReturnResult(sqlmock.NewResult(0, affected))
				err = repo.HGUpdateRechargeSKU(context.Background(), "admin", item)
			}
			if affected == 0 && !errors.Is(err, ErrHGRechargeSKUConflict) || affected == 1 && err != nil {
				t.Fatalf("affected=%d err=%v", affected, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !errors.Is(hgRechargeSKUDBError(&mysql.MySQLError{Number: 1062}), ErrHGRechargeSKUConflict) {
		t.Fatal("duplicate must conflict")
	}
}

func TestHGRechargeSKUPagination(t *testing.T) {
	for _, cursor := range []uint64{0, 10} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		rows := sqlmock.NewRows([]string{"id", "sku_id", "sku_code", "title", "currency", "pay_amount", "coin_amount", "bonus_coin", "total_coin", "status", "sort_order", "start_time", "end_time", "version", "created_at", "updated_at"})
		now := time.Now()
		for _, id := range []int64{9, 8, 7} {
			rows.AddRow(id, "RSKU_test", "six", "six", "CNY", 600, 600, 0, 600, 0, 0, now, nil, 1, now, now)
		}
		if cursor == 0 {
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsPaymentRechargeSKUListFirstSQL)).WithArgs(3).WillReturnRows(rows).RowsWillBeClosed()
		} else {
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsPaymentRechargeSKUListByCursorSQL)).WithArgs(cursor, 3).WillReturnRows(rows).RowsWillBeClosed()
		}
		items, more, err := NewRepository(db).HGListRechargeSKUs(context.Background(), cursor, 2)
		if err != nil || !more || len(items) != 2 || items[1].ID != 8 || items[0].EndTime != nil {
			t.Fatalf("items=%+v more=%v err=%v", items, more, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHGRechargeSKUMigrationAndSQL(t *testing.T) {
	content, err := os.ReadFile("../../../../migrations/000032_create_payment_recharge_sku.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"CREATE TABLE `payment_recharge_sku`", "UNIQUE KEY `uk_payment_recharge_sku_code` (`sku_code`)", "(`is_deleted`,`id` DESC)", "`created_by`", "`updated_by`", "payment.recharge_sku.read", "payment.recharge_sku.write", "super-admin"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, query := range []string{SQLQueriesPackage.UpdateOpsPaymentRechargeSKUSQL, SQLQueriesPackage.DeleteOpsPaymentRechargeSKUSQL} {
		for _, want := range []string{"`sku_id`=?", "`version`=?", "`is_deleted`=0", "`version`=`version`+1"} {
			if !strings.Contains(query, want) {
				t.Fatalf("missing %s", want)
			}
		}
	}
	if !strings.Contains(SQLQueriesPackage.DeleteOpsPaymentRechargeSKUSQL, "SET `is_deleted`=1") {
		t.Fatal("must soft delete")
	}
}

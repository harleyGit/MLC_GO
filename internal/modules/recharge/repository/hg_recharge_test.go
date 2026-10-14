package repository

import (
	"MLC_GO/internal/modules/recharge/model"
	hgqueries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGRechargeReadOnlyOwnerAndCursor(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgTime := time.Date(2026, 10, 10, 0, 0, 0, 123000000, time.UTC)
	hgColumns := []string{"id", "recharge_order_id", "sku_snapshot_title", "currency", "pay_amount", "total_coin", "order_status", "payment_mode", "created_at", "expire_at", "paid_at"}
	hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGRechargeFirstPageSQL)).WithArgs("Owner", "Owner", 51).WillReturnRows(sqlmock.NewRows(hgColumns).AddRow(9, "order", "标题", "CNY", int64(9223372036854775807), int64(9007199254740993), 3, "platform_debug", hgTime, hgTime, hgTime)).RowsWillBeClosed()
	hgRepo := HGNewRepository(hgDB)
	hgItems, hgErr := hgRepo.HGList(context.Background(), "Owner", nil, 50)
	if hgErr != nil || len(hgItems) != 1 || hgItems[0].HGPayAmount != 9223372036854775807 || hgItems[0].HGPaidAt == nil {
		t.Fatalf("%+v %v", hgItems, hgErr)
	}
	hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGRechargeNextPageSQL)).WithArgs("owner", "owner", hgTime, hgTime, uint64(9), 2).WillReturnRows(sqlmock.NewRows(hgColumns)).RowsWillBeClosed()
	if _, hgErr = hgRepo.HGList(context.Background(), "owner", &model.HGCursor{HGCreatedAt: hgTime, HGID: 9}, 1); hgErr != nil {
		t.Fatal(hgErr)
	}
	for _, hgSQL := range []string{hgqueries.HGRechargeFirstPageSQL, hgqueries.HGRechargeNextPageSQL} {
		for _, hgRequired := range []string{"FROM payment_recharge_order FORCE INDEX (idx_payment_recharge_order_user_created_id)", "user_id = ? AND BINARY user_id = BINARY ?", "ORDER BY created_at DESC, id DESC LIMIT ?"} {
			if !strings.Contains(hgSQL, hgRequired) {
				t.Fatal("缺少索引或隔离约束")
			}
		}
		for _, hgForbidden := range []string{"JOIN", "OFFSET", "COUNT", "UPDATE", "INSERT"} {
			if strings.Contains(hgSQL, hgForbidden) {
				t.Fatal("记录读取越界")
			}
		}
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

func TestHGRechargeRepositoryErrors(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgFailure := errors.New("查询失败")
	hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGRechargeFirstPageSQL)).WithArgs("owner", "owner", 51).WillReturnError(hgFailure)
	if _, hgErr = HGNewRepository(hgDB).HGList(context.Background(), "owner", nil, 50); !errors.Is(hgErr, hgFailure) {
		t.Fatal(hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

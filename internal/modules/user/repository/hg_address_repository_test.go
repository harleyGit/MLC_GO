package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGAddressCreateFirstAndLimit(t *testing.T) {
	for _, tc := range []struct {
		count     int
		wantLimit bool
	}{{0, false}, {20, true}} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressLockUserSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressCountSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.count))
		if tc.wantLimit {
			mock.ExpectRollback()
		} else {
			mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGAddressInsertSQL)).WithArgs("ADDR_test", "owner", "张三", "13800138000", "北京", "110000", "110100", "测试路", true).WillReturnResult(sqlmock.NewResult(5, 1))
			mock.ExpectCommit()
		}
		id, err := HGNewAddressRepository(db).HGCreateAddress(context.Background(), "owner", "ADDR_test", UserDtoPackage.HGAddressInput{ReceiverName: "张三", ReceiverPhone: "13800138000", Region: "北京", ProvinceCode: "110000", CityCode: "110100", DetailAddress: "测试路"})
		if tc.wantLimit && !errors.Is(err, ErrHGAddressLimit) || !tc.wantLimit && (err != nil || id != 5) {
			t.Fatalf("count=%d id=%d err=%v", tc.count, id, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestHGAddressSetDefaultRejectsOtherOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressLockUserSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressGetSQL)).WithArgs(int64(9), "owner").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	if err := HGNewAddressRepository(db).HGSetDefaultAddress(context.Background(), "owner", 9); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("其他用户地址应返回不存在: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGAddressDeleteDefaultPromotesNext(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressLockUserSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressGetSQL)).WithArgs(int64(9), "owner").WillReturnRows(sqlmock.NewRows([]string{"id", "address_id", "receiver_name", "receiver_phone", "region", "province_code", "city_code", "detail_address", "is_default"}).AddRow(9, "ADDR_test", "张三", "13800138000", "北京", "110000", "110100", "测试路", true))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGAddressDeleteSQL)).WithArgs(int64(9), "owner").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGAddressPromoteSQL)).WithArgs("owner").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := HGNewAddressRepository(db).HGDeleteAddress(context.Background(), "owner", 9); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGAddressSetDefaultAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressLockUserSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressGetSQL)).WithArgs(int64(9), "owner").WillReturnRows(sqlmock.NewRows([]string{"id", "address_id", "receiver_name", "receiver_phone", "region", "province_code", "city_code", "detail_address", "is_default"}).AddRow(9, "ADDR_test", "张三", "13800138000", "北京", "110000", "110100", "测试路", false))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGAddressClearDefaultSQL)).WithArgs("owner").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGAddressSetDefaultSQL)).WithArgs(int64(9), "owner").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := HGNewAddressRepository(db).HGSetDefaultAddress(context.Background(), "owner", 9); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGAddressUpdateUnchangedStillSucceeds(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	input := UserDtoPackage.HGAddressInput{ReceiverName: "张三", ReceiverPhone: "13800138000", Region: "北京", ProvinceCode: "110000", CityCode: "110100", DetailAddress: "测试路"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressLockUserSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGAddressGetSQL)).WithArgs(int64(9), "owner").WillReturnRows(sqlmock.NewRows([]string{"id", "address_id", "receiver_name", "receiver_phone", "region", "province_code", "city_code", "detail_address", "is_default"}).AddRow(9, "ADDR_test", "张三", "13800138000", "北京", "110000", "110100", "测试路", true))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGAddressUpdateSQL)).WithArgs("张三", "13800138000", "北京", "110000", "110100", "测试路", int64(9), "owner").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := HGNewAddressRepository(db).HGUpdateAddress(context.Background(), "owner", 9, input); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

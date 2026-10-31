package im

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGRegisterDeviceLimit(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()

	hgMock.ExpectBegin()
	hgMock.ExpectQuery(SQL.HGIMLockDeviceOwnerSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"found"}).AddRow(1))
	hgRows := sqlmock.NewRows([]string{"id"})
	for hgIndex := 0; hgIndex < 5; hgIndex++ {
		hgRows.AddRow([]byte{byte(hgIndex)})
	}
	hgMock.ExpectQuery(SQL.HGIMActiveDevicesSQL).WithArgs("user-1").WillReturnRows(hgRows)
	hgMock.ExpectRollback()

	_, hgErr = HGNewService(hgDB, "", nil).HGRegisterDevice(context.Background(), "user-1")
	if !errors.Is(hgErr, HGErrConflict) {
		t.Fatalf("五台设备应返回冲突而不是数据库错误，实际：%v", hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

func TestHGRegisterDeviceBelowLimit(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()

	hgMock.ExpectBegin()
	hgMock.ExpectQuery(SQL.HGIMLockDeviceOwnerSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"found"}).AddRow(1))
	hgRows := sqlmock.NewRows([]string{"id"})
	for hgIndex := 0; hgIndex < 4; hgIndex++ {
		hgRows.AddRow([]byte{byte(hgIndex)})
	}
	hgMock.ExpectQuery(SQL.HGIMActiveDevicesSQL).WithArgs("user-1").WillReturnRows(hgRows)
	hgMock.ExpectExec(SQL.HGIMCreateDeviceSQL).WithArgs(sqlmock.AnyArg(), "user-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	hgMock.ExpectCommit()

	hgCredential, hgErr := HGNewService(hgDB, "", nil).HGRegisterDevice(context.Background(), "user-1")
	if hgErr != nil || len(hgCredential.DeviceID) != 32 || len(hgCredential.Secret) != 64 {
		t.Fatalf("四台设备应允许注册并返回凭据，错误：%v", hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

func TestHGRegisterDeviceDatabaseFailure(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()

	hgFailure := errors.New("数据库不可用")
	hgMock.ExpectBegin()
	hgMock.ExpectQuery(SQL.HGIMLockDeviceOwnerSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"found"}).AddRow(1))
	hgMock.ExpectQuery(SQL.HGIMActiveDevicesSQL).WithArgs("user-1").WillReturnError(hgFailure)
	hgMock.ExpectRollback()

	_, hgErr = HGNewService(hgDB, "", nil).HGRegisterDevice(context.Background(), "user-1")
	if !errors.Is(hgErr, hgFailure) || errors.Is(hgErr, HGErrConflict) {
		t.Fatalf("真实查询错误不得误判为设备上限，实际：%v", hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

func TestHGRegisterDeviceErrorResponse(t *testing.T) {
	for _, hgCase := range []struct {
		hgName   string
		hgError  error
		hgStatus int
		hgCode   int
	}{
		{hgName: "设备上限", hgError: HGErrConflict, hgStatus: http.StatusConflict, hgCode: 100001},
		{hgName: "数据库故障", hgError: sql.ErrConnDone, hgStatus: http.StatusServiceUnavailable, hgCode: 500002},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgResponse := httptest.NewRecorder()
			hgIMFail(hgResponse, httptest.NewRequest(http.MethodPost, "/api/v1/im/device/register", nil), hgCase.hgError)
			if hgResponse.Code != hgCase.hgStatus {
				t.Fatalf("HTTP 状态错误：%d", hgResponse.Code)
			}
			var hgBody struct {
				Code int `json:"code"`
			}
			if hgErr := json.Unmarshal(hgResponse.Body.Bytes(), &hgBody); hgErr != nil {
				t.Fatal(hgErr)
			}
			if hgBody.Code != hgCase.hgCode {
				t.Fatalf("业务错误码错误：%s", hgResponse.Body.String())
			}
		})
	}
}

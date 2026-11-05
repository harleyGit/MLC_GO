package im

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGMediaMissingAndDatabaseFailure(t *testing.T) {
	for _, hgCase := range []struct {
		hgName   string
		hgRead   bool
		hgDBErr  error
		hgStatus int
		hgCode   int
	}{
		{"审核状态不存在", false, sql.ErrNoRows, http.StatusForbidden, 100001},
		{"媒体读取不存在", true, sql.ErrNoRows, http.StatusForbidden, 100001},
		{"审核状态数据库故障", false, sql.ErrConnDone, http.StatusServiceUnavailable, 500002},
		{"媒体读取数据库故障", true, sql.ErrConnDone, http.StatusServiceUnavailable, 500002},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgDB, hgMock, hgErr := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgDB.Close()
			hgID := "fe916aa904bce26589baf25a0e3dae4b"
			hgBytes, _ := hex.DecodeString(hgID)
			hgMock.ExpectQuery(SQL.HGIMMediaGetSQL).WithArgs(hgBytes).WillReturnError(hgCase.hgDBErr)
			hgMedia := HGNewPrivateMedia(hgDB, nil, nil, "")
			if hgCase.hgRead {
				_, hgErr = hgMedia.hgReadable(context.Background(), "user-1", hgID)
			} else {
				_, hgErr = hgMedia.HGMediaStatus(context.Background(), "user-1", hgID)
			}
			if hgCase.hgDBErr == sql.ErrNoRows && !errors.Is(hgErr, HGErrForbidden) || hgCase.hgDBErr != sql.ErrNoRows && !errors.Is(hgErr, hgCase.hgDBErr) {
				t.Fatalf("错误分类不正确：%v", hgErr)
			}
			hgResponse := httptest.NewRecorder()
			hgIMFail(hgResponse, httptest.NewRequest(http.MethodGet, "/api/v1/im/media/read", nil), hgErr)
			var hgBody struct {
				Code int `json:"code"` // 响应协议错误码。
			}
			if hgResponse.Code != hgCase.hgStatus || json.Unmarshal(hgResponse.Body.Bytes(), &hgBody) != nil || hgBody.Code != hgCase.hgCode {
				t.Fatalf("响应不正确：HTTP %d, body %s", hgResponse.Code, hgResponse.Body.String())
			}
			if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

func TestHGMediaStatusOnlyOwner(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgID := "c6b53a3c1474eb711bcf8087cca257c1"
	hgBytes, _ := hex.DecodeString(hgID)
	hgRows := sqlmock.NewRows([]string{"owner_id", "conversation_id", "object_key", "immutable_key", "media_type", "mime_type", "byte_size", "sha256", "state", "width", "height", "duration_ms"}).
		AddRow("uploader", 1, "quarantine/"+hgID, "", "image", "image/png", 128, make([]byte, 32), "verifying", 0, 0, 0)
	hgMock.ExpectQuery(SQL.HGIMMediaGetSQL).WithArgs(hgBytes).WillReturnRows(hgRows)
	_, hgErr = HGNewPrivateMedia(hgDB, nil, nil, "").HGMediaStatus(context.Background(), "recipient", hgID)
	if !errors.Is(hgErr, HGErrForbidden) {
		t.Fatalf("非上传者不得查看审核状态：%v", hgErr)
	}
	if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

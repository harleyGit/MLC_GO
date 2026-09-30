package OpsServicePackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGRechargeSKUValidation(t *testing.T) {
	base := `{"skuCode":"SKU_6","title":"六元","payAmount":600,"coinAmount":600,"bonusCoin":0,"status":0,"sortOrder":0}`
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", base, true},
		{"fraction", strings.Replace(base, `"payAmount":600`, `"payAmount":1.5`, 1), false},
		{"exponent", strings.Replace(base, `"payAmount":600`, `"payAmount":6e2`, 1), false},
		{"string", strings.Replace(base, `"payAmount":600`, `"payAmount":"600"`, 1), false},
		{"null", strings.Replace(base, `"payAmount":600`, `"payAmount":null`, 1), false},
		{"negative", strings.Replace(base, `"bonusCoin":0`, `"bonusCoin":-1`, 1), false},
		{"zero", strings.Replace(base, `"payAmount":600`, `"payAmount":0`, 1), false},
		{"unsafe", strings.Replace(base, `"payAmount":600`, `"payAmount":9007199254740992`, 1), false},
		{"total overflow", strings.Replace(base, `"bonusCoin":0`, `"bonusCoin":9007199254740991`, 1), false},
		{"status", strings.Replace(base, `"status":0`, `"status":2`, 1), false},
		{"code", strings.Replace(base, "SKU_6", "invalid code", 1), false},
		{"title", strings.Replace(base, "六元", strings.Repeat("字", 129), 1), false},
		{"end", strings.TrimSuffix(base, "}") + `,"endTime":"2000-01-01T00:00:00Z"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var req OpsDtoPackage.HGPaymentRechargeSKUCreateRequest
			err := json.Unmarshal([]byte(tc.body), &req)
			if err == nil {
				var itemErr error
				item, itemErr := hgValidateRechargeSKU(req, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
				err = itemErr
				if err == nil && (item.TotalCoin != 600 || item.Currency != "CNY" || item.Version != 1) {
					t.Fatalf("item=%+v", item)
				}
			}
			if (err == nil) != tc.valid {
				t.Fatalf("err=%v valid=%v", err, tc.valid)
			}
		})
	}
}

func TestHGRechargeSKUPage(t *testing.T) {
	for _, tc := range []struct {
		cursor, size string
		limit        int
		valid        bool
	}{{"", "", 20, true}, {"18446744073709551615", "101", 100, true}, {"-1", "20", 0, false}, {"1e3", "20", 0, false}, {"1", "0", 0, false}, {"1", "1.5", 0, false}, {"18446744073709551616", "20", 0, false}} {
		_, limit, err := hgParseRechargeSKUPage(tc.cursor, tc.size)
		if (err == nil) != tc.valid || tc.valid && limit != tc.limit {
			t.Fatalf("%+v: limit=%d err=%v", tc, limit, err)
		}
	}
}

func TestHGRechargeSKUAuthorization(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		rows := sqlmock.NewRows([]string{"allowed"})
		if allowed {
			rows.AddRow(1)
		}
		mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAssetPermissionSQL)).WithArgs("admin", "payment.recharge_sku.write").WillReturnRows(rows)
		s := NewService(OpsRepositoryPackage.NewRepository(db), nil, nil)
		err = s.hgAuthorizeRechargeSKU(context.Background(), "admin", "payment.recharge_sku.write")
		if allowed && err != nil || !allowed && !errors.Is(err, ErrHGOperationsForbidden) {
			t.Fatalf("allowed=%v err=%v", allowed, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHGRechargeSKUCreateAndEmptyList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewService(OpsRepositoryPackage.NewRepository(db), nil, nil)
	var req OpsDtoPackage.HGPaymentRechargeSKUCreateRequest
	if err := json.Unmarshal([]byte(`{"skuCode":"six","title":"six","payAmount":600,"coinAmount":600,"bonusCoin":20,"status":1,"sortOrder":0}`), &req); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAssetPermissionSQL)).WithArgs("admin", "payment.recharge_sku.write").WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOpsPaymentRechargeSKUSQL)).WithArgs(sqlmock.AnyArg(), "six", "six", "CNY", 600, 600, 20, 620, 1, 0, sqlmock.AnyArg(), nil, "admin", "admin").WillReturnResult(sqlmock.NewResult(1, 1))
	id, err := s.HGCreateRechargeSKU(context.Background(), "admin", req)
	if err != nil || !strings.HasPrefix(id, "RSKU_") {
		t.Fatalf("id=%s err=%v", id, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAssetPermissionSQL)).WithArgs("admin", "payment.recharge_sku.read").WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsPaymentRechargeSKUListFirstSQL)).WithArgs(21).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	page, err := s.HGListRechargeSKUs(context.Background(), "admin", "", "")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(page)
	if err != nil || string(encoded) != `{"list":[],"nextCursor":"","hasMore":false,"total":-1}` {
		t.Fatalf("page=%s err=%v", encoded, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

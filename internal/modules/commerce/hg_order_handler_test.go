package commerce

import (
	"github.com/DATA-DOG/go-sqlmock"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHGCommercePaymentAndShipmentRemainUnavailable(t *testing.T) {
	h := HGNewHandler(nil, nil)
	for _, test := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"payment", h.HGOrderPayment}, {"shipment", h.HGOrderShipment}, {"reserve", h.HGOrderCreate}, {"inventory", h.HGInventoryAdjust},
	} {
		t.Run(test.name, func(t *testing.T) {
			writer := httptest.NewRecorder()
			test.handler(writer, httptest.NewRequest(http.MethodPost, "/api/v1/commerce/"+test.name, nil))
			if writer.Code != http.StatusServiceUnavailable {
				t.Fatalf("未开启交易或渠道必须拒绝写入：%d", writer.Code)
			}
		})
	}
}

func TestHGCommerceTradeWritesDenyPreAndProdWithoutDatabaseAccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := HGNewHandler(nil, nil).HGWithOrderStore(HGNewOrderStore(db))
	for _, env := range []string{"pre", "prod"} {
		t.Setenv("SERVER_ENV", env)
		t.Setenv("MLC_COMMERCE_DEBUG_TRADING", "true")
		for _, handler := range []http.HandlerFunc{h.HGOrderCreate, h.HGInventoryAdjust, h.HGOrderCancel} {
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest(http.MethodPost, "/api/v1/commerce/orders", nil))
			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("%s 环境不得访问交易表：%d", env, w.Code)
			}
		}
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

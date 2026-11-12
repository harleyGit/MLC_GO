package im

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGRPCProxyIdentityAndPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	secret := strings.Repeat("x", 40)
	device := strings.Repeat("a", 32)
	credential := strings.Repeat("b", 64)
	id, _ := hex.DecodeString(device)
	key, _ := hex.DecodeString(credential)
	hash := sha256.Sum256(key)
	handler := HGNewRPCHandler(HGNewService(db, "test.im", nil), nil, secret)
	request := func(proxySecret, user, deviceID, method string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"user": user, "transport": "websocket", "method": method,
			"data": map[string]any{"deviceId": deviceID, "secret": credential, "after": 0}})
		r := httptest.NewRequest(http.MethodPost, "/internal/im/centrifugo/rpc", strings.NewReader(string(body)))
		r.Header.Set("X-IM-RPC-Secret", proxySecret)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := request("bad", "im/"+device, device, "im:pending"); w.Code != http.StatusForbidden {
		t.Fatalf("代理密钥缺失应拒绝: %d", w.Code)
	}
	if w := request(secret, "im/"+device, strings.Repeat("c", 32), "im:pending"); !strings.Contains(w.Body.String(), `"code":403`) {
		t.Fatalf("设备 subject 不匹配应拒绝: %s", w.Body.String())
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGIMRPCDeviceOwnerSQL)).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user1"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGIMDeviceCredentialSQL)).WithArgs(id, "user1").WillReturnRows(sqlmock.NewRows([]string{"credential_hash"}).AddRow(hash[:]))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGIMDeviceCredentialSQL)).WithArgs(id, "user1").WillReturnRows(sqlmock.NewRows([]string{"credential_hash"}).AddRow(hash[:]))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGIMPendingDeviceSQL)).WithArgs(id, "user1", uint64(0), 100).WillReturnRows(sqlmock.NewRows([]string{"message_id", "conversation_id", "seq"}).AddRow(12, 34, 2))
	w := request(secret, "im/"+device, device, "im:pending")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"messageId":"12"`) {
		t.Fatalf("本设备待确认补拉失败: %d %s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

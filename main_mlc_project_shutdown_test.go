//go:build production

package main

import (
	IM "MLC_GO/internal/modules/im"
	IMTransport "MLC_GO/internal/modules/im/transport"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHGWaitServeErrorsCollectsFailures(t *testing.T) {
	serveErr := make(chan error, 2)
	serveErr <- nil
	serveErr <- errors.New("realtime failed")
	if err := hgWaitServeErrors(serveErr, 2, time.Second); err == nil || !strings.Contains(err.Error(), "realtime failed") {
		t.Fatalf("hgWaitServeErrors() error = %v", err)
	}
}

// 外部能力尚无隔离实例；这些负向协议用例仅使用内存 HTTP 服务，不写数据库或实际网关。
func TestHGIMCentrifugoRejectsUnconfirmedPublish(t *testing.T) {
	for _, hgBody := range []string{`{"error":{"code":100}}`, `{"result":null}`, `{}`, `{"result":[]}`} {
		t.Run(hgBody, func(t *testing.T) {
			hgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/publish" || r.Header.Get("X-API-Key") != strings.Repeat("k", 16) {
					t.Error("网关请求契约不匹配")
				}
				_, _ = w.Write([]byte(hgBody))
			}))
			defer hgServer.Close()
			hgGateway, hgErr := IMTransport.HGNewCentrifugo(hgServer.URL, strings.Repeat("k", 16), strings.Repeat("s", 32))
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgGateway.HGClose()
			if hgErr = hgGateway.HGPublish(context.Background(), strings.Repeat("a", 32), json.RawMessage(`{"eventType":"message.committed"}`)); hgErr == nil {
				t.Fatal("HTTP 200 的缺失/失败发布结果被当成成功")
			}
		})
	}
}

func TestHGIMLiveKitWebhookRejectsTamperingAndWrongIdentity(t *testing.T) {
	hgSecret := strings.Repeat("s", 32)
	hgLiveKit, hgErr := IMTransport.HGNewLiveKit("http://127.0.0.1:7880", "fixture", hgSecret)
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgLiveKit.HGClose()
	hgBody := []byte(`{"id":"event-1","event":"room_finished","room":{"name":"im_01234567890123456789012345678901","sid":"RM_fixture"}}`)
	hgHash := sha256.Sum256(hgBody)
	for _, hgCase := range []struct {
		hgName   string    // 负向条件。
		hgIssuer string    // JWT 签发者。
		hgExpiry time.Time // Token 失效时间。
		hgTamper bool      // 是否更改签名后的正文。
		hgAccept bool      // 是否应通过认证。
	}{
		{"valid", "fixture", time.Now().Add(time.Minute), false, true},
		{"tampered", "fixture", time.Now().Add(time.Minute), true, false},
		{"wrong issuer", "other", time.Now().Add(time.Minute), false, false},
		{"expired", "fixture", time.Now().Add(-time.Minute), false, false},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgToken, hgErr := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": hgCase.hgIssuer, "exp": hgCase.hgExpiry.Unix(), "sha256": base64.StdEncoding.EncodeToString(hgHash[:])}).SignedString([]byte(hgSecret))
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			hgPayload := append([]byte(nil), hgBody...)
			if hgCase.hgTamper {
				hgPayload = append(hgPayload, ' ')
			}
			_, hgErr = hgLiveKit.HGVerifyWebhook(hgToken, hgPayload)
			if (hgErr == nil) != hgCase.hgAccept {
				t.Fatalf("验签结果与预期不符: %v", hgErr)
			}
		})
	}
}

func TestHGIMPrivateUploadCannotSignSealedOrUnboundedObjects(t *testing.T) {
	hgStorage, hgErr := IMTransport.HGNewSeaweedFS("http://127.0.0.1:8333", "us-east-1", "im-private", "fixture", strings.Repeat("s", 32))
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgStorage.HGClose()
	for _, hgKey := range []string{"sealed/" + strings.Repeat("a", 32), "quarantine/../sealed/" + strings.Repeat("a", 32), "quarantine/a?acl=public-read"} {
		if _, hgErr = hgStorage.HGUploadURL(hgKey, "image/png", 10); hgErr == nil {
			t.Fatal("越过隔离对象路径边界")
		}
	}
	if _, hgErr = hgStorage.HGUploadURL("quarantine/"+strings.Repeat("a", 32), "image/png", 129<<20); hgErr == nil {
		t.Fatal("未拒绝超限上传")
	}
	hgLink, hgErr := hgStorage.HGUploadURL("quarantine/"+strings.Repeat("a", 32), "image/png", 10)
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	hgURL, hgErr := url.Parse(hgLink)
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	if hgURL.Query().Get("X-Amz-SignedHeaders") != "content-length;content-type;host;x-amz-acl" {
		t.Fatal("上传未签名大小/类型/私有 ACL")
	}
	if hgURL.Query().Get("X-Amz-Expires") != "600" {
		t.Fatal("上传授权有效期不符")
	}
}

func TestHGWaitServeErrorsHasBoundedTimeout(t *testing.T) {
	serveErr := make(chan error)
	startedAt := time.Now()
	err := hgWaitServeErrors(serveErr, 2, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "2 server listeners did not stop") {
		t.Fatalf("hgWaitServeErrors() timeout error = %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("hgWaitServeErrors() blocked for %s", elapsed)
	}
}

func TestHGIMPrivateReadValidatesByteRanges(t *testing.T) {
	for _, hgCase := range []struct {
		hgName         string // 范围契约场景。
		hgRange        string // 客户端单段 Range。
		hgContentRange string // 存储服务 Content-Range。
		hgStatus       int    // 存储服务 HTTP 状态。
		hgEncoding     string // 不允许改变封存对象的编码。
		hgAccept       bool   // 是否允许返回内容。
	}{
		{"full", "", "", 200, "", true},
		{"bounded", "bytes=2-4", "bytes 2-4/10", 206, "", true},
		{"open-ended", "bytes=7-", "bytes 7-9/10", 206, "", true},
		{"clipped", "bytes=7-100", "bytes 7-9/10", 206, "", true},
		{"missing-range", "bytes=2-4", "", 206, "", false},
		{"wrong-start", "bytes=2-4", "bytes 1-3/10", 206, "", false},
		{"wrong-end", "bytes=2-4", "bytes 2-5/10", 206, "", false},
		{"wrong-length", "bytes=2-3", "bytes 2-3/10", 206, "", false},
		{"unknown-total", "bytes=2-4", "bytes 2-4/*", 206, "", false},
		{"oversize-total", "bytes=2-4", "bytes 2-4/314572801", 206, "", false},
		{"end-outside-object", "bytes=8-10", "bytes 8-10/10", 206, "", false},
		{"unexpected-partial", "", "bytes 0-2/10", 206, "", false},
		{"range-in-full", "", "bytes 0-2/10", 200, "", false},
		{"encoded", "bytes=2-4", "bytes 2-4/10", 206, "gzip", false},
		{"reversed", "bytes=4-2", "bytes 2-4/10", 206, "", false},
		{"multiple", "bytes=0-1,3-4", "bytes 0-2/10", 206, "", false},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Accept-Encoding") != "identity" || r.Header.Get("Range") != hgCase.hgRange {
					t.Error("私有对象请求未保留原始字节范围")
				}
				w.Header().Set("Content-Length", "3")
				w.Header().Set("Content-Range", hgCase.hgContentRange)
				w.Header().Set("Content-Encoding", hgCase.hgEncoding)
				w.WriteHeader(hgCase.hgStatus)
				_, _ = io.WriteString(w, "abc")
			}))
			defer hgServer.Close()
			hgStorage, hgErr := IMTransport.HGNewSeaweedFS(hgServer.URL, "us-east-1", "im-private", "fixture", strings.Repeat("s", 32))
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgStorage.HGClose()
			hgResponse, hgErr := hgStorage.HGOpen(context.Background(), "sealed/"+strings.Repeat("a", 32), hgCase.hgRange)
			if (hgErr == nil) != hgCase.hgAccept {
				t.Fatalf("范围响应结果不符: %v", hgErr)
			}
			if hgResponse != nil {
				defer hgResponse.Body.Close()
				hgBody, hgErr := io.ReadAll(hgResponse.Body)
				if hgErr != nil || string(hgBody) != "abc" {
					t.Fatal("未保留封存内容")
				}
			}
		})
	}
}

func TestHGIMLiveKitDeleteRequiresProtocolConfirmation(t *testing.T) {
	for _, hgCase := range []struct {
		hgName   string // 正常删除、幂等删除或代理故障。
		hgStatus int    // HTTP 状态。
		hgBody   string // Twirp JSON 或代理正文。
		hgAccept bool   // 是否确认房间已不存在。
	}{
		{"deleted", 200, `{}`, true},
		{"null-success", 200, `null`, false},
		{"proxy-success", 200, "upstream welcome page", false},
		{"absent", 404, `{"code":"not_found","msg":"room absent"}`, true},
		{"proxy-404", 404, "upstream route missing", false},
		{"empty-404", 404, `{}`, false},
		{"wrong-code", 404, `{"code":"bad_route"}`, false},
		{"unavailable", 503, `{"code":"unavailable"}`, false},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/twirp/livekit.RoomService/DeleteRoom" {
					t.Error("删除房间路由错误")
				}
				w.WriteHeader(hgCase.hgStatus)
				_, _ = io.WriteString(w, hgCase.hgBody)
			}))
			defer hgServer.Close()
			hgLiveKit, hgErr := IMTransport.HGNewLiveKit(hgServer.URL, "fixture", strings.Repeat("s", 32))
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgLiveKit.HGClose()
			hgErr = hgLiveKit.HGDeleteRoom(context.Background(), "im_"+strings.Repeat("a", 32))
			if (hgErr == nil) != hgCase.hgAccept {
				t.Fatalf("房间删除确认不符: %v", hgErr)
			}
		})
	}
}

func TestHGIMCallTokenRechecksAuthorityAfterRoomCreation(t *testing.T) {
	for _, hgCase := range []string{"unchanged-window", "revoked-device", "removed-member", "expired-call"} {
		t.Run(hgCase, func(t *testing.T) {
			hgDB, hgMock, hgErr := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgDB.Close()
			hgID := bytes.Repeat([]byte{1}, 16)
			hgDevice := bytes.Repeat([]byte{2}, 16)
			hgSecret := bytes.Repeat([]byte{3}, 32)
			hgHash := sha256.Sum256(hgSecret)
			hgRows := func() *sqlmock.Rows {
				return sqlmock.NewRows([]string{"id", "conversation_id", "caller_id", "callee_id", "caller_device", "callee_device", "media_type", "state", "version", "expires_at_ms"}).AddRow(hgID, 1, "caller", "callee", hgDevice, nil, "audio", "accepted", 2, time.Now().Add(time.Minute).UnixMilli())
			}
			hgMock.ExpectQuery(SQL.HGIMDeviceCredentialSQL).WithArgs(hgDevice, "caller").WillReturnRows(sqlmock.NewRows([]string{"credential_hash"}).AddRow(hgHash[:]))
			hgMock.ExpectQuery(SQL.HGIMCallGetSQL).WithArgs(hgID).WillReturnRows(hgRows())
			hgMock.ExpectQuery(SQL.HGIMConversationMemberSQL).WithArgs("1", "caller").WillReturnRows(sqlmock.NewRows([]string{"member"}).AddRow(1))
			hgMock.ExpectBegin()
			hgMock.ExpectQuery(SQL.HGIMCallLockSQL).WithArgs(hgID).WillReturnRows(hgRows())
			hgMemberRows := sqlmock.NewRows([]string{"member"})
			if hgCase != "removed-member" {
				hgMemberRows.AddRow(1)
			}
			hgMock.ExpectQuery(SQL.HGIMCallMemberSQL).WithArgs("1", "caller").WillReturnRows(hgMemberRows)
			if hgCase != "removed-member" {
				hgActiveRows := sqlmock.NewRows([]string{"active"})
				if hgCase != "revoked-device" {
					hgActiveRows.AddRow(1)
				}
				hgMock.ExpectQuery(SQL.HGIMCallDeviceActiveSQL).WithArgs(hgDevice, "caller").WillReturnRows(hgActiveRows)
			}
			if hgCase == "unchanged-window" || hgCase == "expired-call" {
				hgMock.ExpectQuery(SQL.HGIMCallExpiredFlagSQL).WithArgs(hgID).WillReturnRows(sqlmock.NewRows([]string{"expired"}).AddRow(hgCase == "expired-call"))
			}
			if hgCase == "unchanged-window" {
				hgMock.ExpectExec(SQL.HGIMCallTokenWindowSQL).WithArgs(hgID).WillReturnResult(sqlmock.NewResult(0, 0))
				hgMock.ExpectExec(SQL.HGIMCallSetRoomSIDSQL).WithArgs("RM_fixture", hgID).WillReturnResult(sqlmock.NewResult(0, 0))
				hgMock.ExpectCommit()
			} else {
				hgMock.ExpectRollback()
			}
			hgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/twirp/livekit.RoomService/CreateRoom" {
					t.Error("创建房间路由错误")
				}
				_, _ = io.WriteString(w, `{"sid":"RM_fixture"}`)
			}))
			defer hgServer.Close()
			hgLiveKit, hgErr := IMTransport.HGNewLiveKit(hgServer.URL, "fixture", strings.Repeat("s", 32))
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgLiveKit.HGClose()
			hgCalls := IM.HGNewCalls(IM.HGNewService(hgDB, "im-fixture", nil), hgLiveKit, "ws://127.0.0.1:7880")
			hgResult, hgErr := hgCalls.HGToken(context.Background(), "caller", hex.EncodeToString(hgID), IM.HGDeviceCredential{DeviceID: hex.EncodeToString(hgDevice), Secret: hex.EncodeToString(hgSecret)})
			if hgCase == "unchanged-window" {
				if hgErr != nil || hgResult["token"] == "" || hgResult["token"] == nil {
					t.Fatalf("未变更窗口应仍可取令牌: %v", hgErr)
				}
			} else {
				hgWant := IM.HGErrForbidden
				if hgCase == "expired-call" {
					hgWant = IM.HGErrConflict
				}
				if !errors.Is(hgErr, hgWant) || hgResult != nil {
					t.Fatalf("授权失效后仍签发令牌: %v", hgErr)
				}
			}
			if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

func TestHGIMDeliveryDoesNotCountLostLeaseAsExhausted(t *testing.T) {
	for _, hgUpdated := range []int64{0, 1} {
		t.Run(strconv.FormatInt(hgUpdated, 10), func(t *testing.T) {
			hgDB, hgMock, hgErr := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgDB.Close()
			hgDevice := bytes.Repeat([]byte{1}, 16)
			hgMock.ExpectBegin()
			hgMock.ExpectQuery(SQL.HGIMJobClaimSQL).WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"id", "device_id", "user_id", "message_id", "payload", "attempts"}).AddRow(1, hgDevice, "caller", 0, []byte(`{}`), 12))
			hgMock.ExpectExec(SQL.HGIMJobLeaseSQL).WithArgs(sqlmock.AnyArg(), 1).WillReturnResult(sqlmock.NewResult(0, 1))
			hgMock.ExpectCommit()
			hgMock.ExpectQuery(SQL.HGIMJobDeviceActiveSQL).WithArgs(hgDevice, "caller").WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(1))
			hgMock.ExpectExec(SQL.HGIMJobFinishSQL).WithArgs(3, 256, 1, sqlmock.AnyArg()).WillReturnResult(driver.RowsAffected(hgUpdated))
			// 已耗尽任务不应访问网关；仅验证租约失效不会记为完成。
			hgRealtime := IM.HGNewRealtime(hgDB, nil, "im-fixture")
			hgErr = hgRealtime.HGDispatchOnce(context.Background())
			if (hgErr == nil) != (hgUpdated == 1) {
				t.Fatalf("租约完成结果不符: %v", hgErr)
			}
			var hgMetrics bytes.Buffer
			hgRealtime.HGWriteMetrics(&hgMetrics)
			if !strings.Contains(hgMetrics.String(), "mlc_im_delivery_exhausted_total "+strconv.FormatInt(hgUpdated, 10)+"\n") {
				t.Fatal("丢失租约被统计为完成耗尽")
			}
			if hgErr = hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

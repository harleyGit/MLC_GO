package media

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type hgFakeStore struct {
	size   int64  // Head 返回的真实对象字节数，测试期间不访问外部服务。
	called bool   // 标记是否在短事务外发生远端调用。
	key    string // 最近一次签名键，验证与审核器对象键契约一致。
}

func (f *hgFakeStore) HGSignUpload(_ context.Context, key string, _ string, _ int64) (string, error) {
	f.called = true
	f.key = key
	return "https://upload.example.test/opaque", nil
}
func (f *hgFakeStore) HGHead(context.Context, string) (int64, error) {
	f.called = true
	return f.size, nil
}

func hgTestRequest() HGUploadRequest {
	return HGUploadRequest{Purpose: "user_avatar", MIME: "image/jpeg", Size: 123, SHA256: strings.Repeat("a", 64), IdempotencyKey: "once_1"}
}

func hgMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func TestHGInitIdempotentReplayAndPayloadConflict(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{true: "载荷冲突", false: "相同请求"}[conflict], func(t *testing.T) {
			db, mock := hgMockDB(t)
			store := &hgFakeStore{}
			service, _ := HGNewService(db, store, "mlc-temp")
			req := hgTestRequest()
			purpose := req.Purpose
			if conflict {
				purpose = "shop_logo"
			}
			mock.ExpectBegin()
			mock.ExpectExec("INSERT IGNORE INTO media_upload_quota").WithArgs("owner").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT owner_id FROM media_upload_quota").WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"owner_id"}).AddRow("owner"))
			mock.ExpectQuery("SELECT upload_id, asset_id, purpose").WithArgs("owner", req.IdempotencyKey).WillReturnRows(sqlmock.NewRows([]string{"upload_id", "asset_id", "purpose", "mime_type", "expected_size", "expected_sha256", "object_key", "status", "expires_at"}).AddRow("upload", "asset", purpose, req.MIME, req.Size, req.SHA256, "quarantine/019ac000000070008000000000000001", "pending", time.Now().Add(time.Hour)))
			if conflict {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			result, err := service.HGInit(context.Background(), "owner", req)
			if conflict {
				if !errors.Is(err, HGErrConflict) || store.called {
					t.Fatalf("冲突请求不得获签名: %v", err)
				}
			} else if err != nil || result.UploadURL == "" || result.AssetID != "asset" {
				t.Fatalf("重试应复用会话: %+v %v", result, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGInitPersistsBeforeSigning(t *testing.T) {
	db, mock := hgMockDB(t)
	store := &hgFakeStore{}
	service, _ := HGNewService(db, store, "mlc-temp")
	req := hgTestRequest()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO media_upload_quota").WithArgs("owner").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT owner_id FROM media_upload_quota").WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"owner_id"}).AddRow("owner"))
	mock.ExpectQuery("SELECT upload_id, asset_id, purpose").WithArgs("owner", req.IdempotencyKey).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT expected_size FROM media_upload_sessions").WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"expected_size"}))
	mock.ExpectExec("INSERT INTO media_assets").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO media_upload_sessions").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	result, err := service.HGInit(context.Background(), "owner", req)
	if err != nil || result.Status != "pending" || result.UploadURL == "" || !store.called {
		t.Fatalf("持久化成功后才应签发隔离桶授权: %+v %v", result, err)
	}
	if store.key != "quarantine/"+strings.ReplaceAll(result.AssetID, "-", "") || !hgQuarantineKey.MatchString(store.key) {
		t.Fatal("新申请的随机隔离键未匹配审核器契约")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGCompleteRejectsSizeMismatchWithoutStateChange(t *testing.T) {
	db, mock := hgMockDB(t)
	service, _ := HGNewService(db, &hgFakeStore{size: 122}, "mlc-temp")
	id := "019ac000-0000-7000-8000-000000000001"
	mock.ExpectQuery("SELECT asset_id, object_key, expected_size").WithArgs(id, "owner").WillReturnRows(sqlmock.NewRows([]string{"asset_id", "object_key", "expected_size", "status", "expires_at"}).AddRow(id, "quarantine/019ac000000070008000000000000001", 123, "pending", time.Now().Add(time.Hour)))
	if _, err := service.HGComplete(context.Background(), "owner", id); !errors.Is(err, HGErrConflict) {
		t.Fatalf("对象长度不匹配仍确认：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGCompleteDoesNotRevealAnotherOwner(t *testing.T) {
	db, mock := hgMockDB(t)
	store := &hgFakeStore{size: 123}
	service, _ := HGNewService(db, store, "mlc-temp")
	id := "019ac000-0000-7000-8000-000000000001"
	mock.ExpectQuery("SELECT asset_id, object_key, expected_size").WithArgs(id, "other").WillReturnError(sql.ErrNoRows)
	if _, err := service.HGComplete(context.Background(), "other", id); !errors.Is(err, HGErrNotFound) || store.called {
		t.Fatalf("跨用户查询应失败且不得访问 S3：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

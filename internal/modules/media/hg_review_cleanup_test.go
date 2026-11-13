package media

import (
	Transport "MLC_GO/internal/modules/im/transport"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGReviewValidRejectsForgedSealedObject(t *testing.T) {
	id := "019ac000-0000-7000-8000-000000000001"
	job := hgReviewJob{asset: id, purpose: "user_avatar", key: "quarantine/" + strings.ReplaceAll(id, "-", ""), mime: "image/jpeg", sha: strings.Repeat("a", 64), size: 123}
	result := Transport.HGReviewResult{Status: "ready", SealedKey: "sealed/" + strings.ReplaceAll(id, "-", ""), MIME: job.mime, SHA256: job.sha, Size: job.size, Width: 128, Height: 128}
	if !hgReviewValid(job, result) {
		t.Fatal("有效封存证明应通过结构校验")
	}
	for name, mutate := range map[string]func(*Transport.HGReviewResult){
		"伪造键":   func(r *Transport.HGReviewResult) { r.SealedKey = "sealed/00000000000000000000000000000000" },
		"摘要不一致": func(r *Transport.HGReviewResult) { r.SHA256 = strings.Repeat("b", 64) },
		"类型不一致": func(r *Transport.HGReviewResult) { r.MIME = "text/html" },
		"长度不一致": func(r *Transport.HGReviewResult) { r.Size++ },
		"无效宽度":  func(r *Transport.HGReviewResult) { r.Width = 0 },
		"错误状态":  func(r *Transport.HGReviewResult) { r.Status = "pending" },
	} {
		t.Run(name, func(t *testing.T) {
			got := result
			mutate(&got)
			if hgReviewValid(job, got) {
				t.Fatal("不得信任伪造的审核结果")
			}
		})
	}
}

type hgCaptureLease struct{ value []byte } // 测试用租约接收器，确认数据库 CAS 与原任务一致。
func (c *hgCaptureLease) Match(v driver.Value) bool {
	bytes, ok := v.([]byte)
	if !ok || len(bytes) != 16 {
		return false
	}
	c.value = append([]byte(nil), bytes...)
	return true
}

type hgReviewStub struct {
	run func(id, bucket, key, sha, mime string, size int64) (Transport.HGReviewResult, error)
} // 受信审核服务替身。
func (r hgReviewStub) HGReview(_ context.Context, id, bucket, key, sha, mime string, size int64) (Transport.HGReviewResult, error) {
	return r.run(id, bucket, key, sha, mime, size)
}

func TestHGReviewOnceRequiresMatchingSealBeforeReady(t *testing.T) {
	asset := "019ac000-0000-7000-8000-000000000001"
	key := "quarantine/" + strings.ReplaceAll(asset, "-", "")
	sealed := "sealed/" + strings.ReplaceAll(asset, "-", "")
	sha := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name  string
		mime  string
		state string
	}{
		{"可信审核通过", "image/jpeg", "ready"},
		{"审核类型伪造", "text/html", "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := hgMockDB(t)
			service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
			lease := new(hgCaptureLease)
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaReviewClaimSQL)).WillReturnRows(sqlmock.NewRows([]string{"upload_id", "asset_id", "purpose", "object_key", "mime_type", "expected_size", "expected_sha256", "attempts"}).AddRow("upload", asset, "user_avatar", key, "image/jpeg", int64(123), sha, 0))
			mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaReviewLeaseSQL)).WithArgs(lease, "upload").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			reviewer := hgReviewStub{run: func(id, bucket, object, hash, mime string, size int64) (Transport.HGReviewResult, error) {
				if id != strings.ReplaceAll(asset, "-", "") || bucket != "mlc-temp" || object != key || hash != sha || mime != "image/jpeg" || size != 123 {
					t.Fatal("审核调用与会话绑定不一致")
				}
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaReviewLockSQL)).WithArgs("upload").WillReturnRows(sqlmock.NewRows([]string{"status", "lease_token"}).AddRow("processing", lease.value))
				mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaReviewFinishSQL)).WithArgs(tc.state, 30, "upload", lease.value).WillReturnResult(sqlmock.NewResult(0, 1))
				if tc.state == "ready" {
					mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaReviewAssetSQL)).WithArgs("ready", sealed, 128, 128, int64(0), asset).WillReturnResult(sqlmock.NewResult(0, 1))
				} else {
					mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaReviewAssetSQL)).WithArgs("failed", nil, nil, nil, nil, asset).WillReturnResult(sqlmock.NewResult(0, 1))
				}
				mock.ExpectCommit()
				return Transport.HGReviewResult{Status: "ready", SealedKey: sealed, SHA256: sha, Size: 123, MIME: tc.mime, Width: 128, Height: 128}, nil
			}}
			if err := service.HGEnableReview(reviewer, &hgFakeSealed{}); err != nil {
				t.Fatal(err)
			}
			if err := service.HGReviewOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGExpirePreservesReferencedAsset(t *testing.T) {
	db, mock := hgMockDB(t)
	service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
	service.sealed = &hgFakeSealed{}
	service.expireCursor.Store(3)
	asset := "019ac000-0000-7000-8000-000000000001"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaExpireReadySQL)).WillReturnRows(sqlmock.NewRows([]string{"upload_id", "asset_id", "object_key", "status"}).AddRow("upload", asset, "quarantine/"+strings.ReplaceAll(asset, "-", ""), "ready"))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaExpireAssetLockSQL)).WithArgs(asset).WillReturnRows(sqlmock.NewRows([]string{"status", "sealed_key"}).AddRow("ready", "sealed/"+strings.ReplaceAll(asset, "-", "")))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaHasRefSQL)).WithArgs(asset).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaExpireAttachSQL)).WithArgs("upload").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := service.HGExpireOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGExpireQueuesBothQuarantineAndPotentialSeal(t *testing.T) {
	db, mock := hgMockDB(t)
	service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
	service.sealed = &hgFakeSealed{}
	asset := "019ac000-0000-7000-8000-000000000001"
	key := "quarantine/" + strings.ReplaceAll(asset, "-", "")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaExpireClaimSQL)).WillReturnRows(sqlmock.NewRows([]string{"upload_id", "asset_id", "object_key", "status"}).AddRow("upload", asset, key, "pending"))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaExpireAssetLockSQL)).WithArgs(asset).WillReturnRows(sqlmock.NewRows([]string{"status", "sealed_key"}).AddRow("pending", nil))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaHasRefSQL)).WithArgs(asset).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaExpireAssetSQL)).WithArgs(asset).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaExpireSessionSQL)).WithArgs("upload").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaDeleteQueueSQL)).WithArgs(asset, "mlc-temp", key).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaDeleteQueueSQL)).WithArgs(asset, "mlc-temp", "sealed/"+strings.ReplaceAll(asset, "-", "")).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	if err := service.HGExpireOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGReferenceNeedsBusinessAuthority(t *testing.T) {
	db, _ := hgMockDB(t)
	service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
	cmd := HGReferenceCommand{AssetID: "019ac000-0000-7000-8000-000000000001", OwnerID: "owner", Purpose: "user_avatar", BizType: "user", BizID: "owner", Slot: "avatar"}
	if err := service.HGReplaceReferenceTx(context.Background(), nil, cmd, nil); !errors.Is(err, HGErrInvalid) {
		t.Fatalf("不得无权威事务绑定资源：%v", err)
	}
	cmd.BizType = "product"
	if err := hgValidateReference(cmd); !errors.Is(err, HGErrInvalid) {
		t.Fatalf("不得跨用途绑定资源：%v", err)
	}
}

func TestHGReferenceOnlyBindsReadyAssetInBusinessTransaction(t *testing.T) {
	asset := "019ac000-0000-7000-8000-000000000001"
	for _, status := range []string{"pending", "ready"} {
		t.Run(status, func(t *testing.T) {
			db, mock := hgMockDB(t)
			service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
			cmd := HGReferenceCommand{AssetID: asset, OwnerID: "owner", Purpose: "user_avatar", BizType: "user", BizID: "owner", Slot: "avatar"}
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaReferenceSlotSQL)).WithArgs("user", "owner", "avatar").WillReturnError(sql.ErrNoRows)
			mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaReferenceSessionSQL)).WithArgs(asset).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(status))
			mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaReferenceAssetSQL)).WithArgs(asset).WillReturnRows(sqlmock.NewRows([]string{"owner_id", "purpose", "status"}).AddRow("owner", "user_avatar", status))
			if status == "ready" {
				mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaReferenceInsertSQL)).WithArgs(asset, "owner", "user", "owner", "avatar").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaReferenceAttachSQL)).WithArgs(asset).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
			if err != nil {
				t.Fatal(err)
			}
			called := false
			err = service.HGReplaceReferenceTx(context.Background(), tx, cmd, func(context.Context, *sql.Tx) error { called = true; return nil })
			if !called {
				t.Fatal("未调用权威业务资格回调")
			}
			if status == "ready" {
				if err != nil {
					t.Fatal(err)
				}
				if err = tx.Commit(); err != nil {
					t.Fatal(err)
				}
			} else {
				if !errors.Is(err, HGErrConflict) {
					t.Fatalf("未审文件应拒绝：%v", err)
				}
				_ = tx.Rollback()
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGOwnerDownloadOnlyReadyAndOwner(t *testing.T) {
	id := "019ac000-0000-7000-8000-000000000001"
	for _, tc := range []struct {
		name, state string
		key         any
		want        bool
	}{
		{"未审核", "processing", nil, false},
		{"未封存", "ready", nil, false},
		{"正确封存", "ready", "sealed/" + strings.ReplaceAll(id, "-", ""), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := hgMockDB(t)
			service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
			service.read = &hgFakeSealed{}
			mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaOwnerDownloadSQL)).WithArgs(id, "owner").WillReturnRows(sqlmock.NewRows([]string{"purpose", "status", "sealed_key"}).AddRow("user_avatar", tc.state, tc.key))
			link, err := service.HGOwnerDownload(context.Background(), "owner", id)
			if tc.want && (err != nil || link == "") || !tc.want && (!errors.Is(err, HGErrNotFound) || link != "") {
				t.Fatalf("私有授权结果错误：%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	db, mock := hgMockDB(t)
	service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
	service.read = &hgFakeSealed{}
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaOwnerDownloadSQL)).WithArgs(id, "other").WillReturnError(sql.ErrNoRows)
	if _, err := service.HGOwnerDownload(context.Background(), "other", id); !errors.Is(err, HGErrNotFound) {
		t.Fatalf("跨用户读取应拒绝：%v", err)
	}
	for _, purpose := range []string{"chat_image", "chat_video", "user_identity", "support_evidence", "video_source"} {
		if hgOwnerReadPurpose(purpose) {
			t.Fatalf("敏感用途不得绕过业务权限下载: %s", purpose)
		}
	}
}

func TestHGDeleteStaleLeaseCannotFinalize(t *testing.T) {
	db, mock := hgMockDB(t)
	service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
	job := hgDeleteJob{id: 9, asset: "019ac000-0000-7000-8000-000000000001", lease: []byte("stale-lease-1234")}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaDeleteFinishSQL)).WithArgs("done", 30, job.id, job.lease).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	if err := service.hgFinishDelete(context.Background(), job, "done"); !errors.Is(err, HGErrConflict) {
		t.Fatalf("旧租约不得提交删除：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type hgFakeSealed struct {
	deleted []string // 测试记录被删除的对象键，不操作真实 S3。
}

func (f *hgFakeSealed) HGHeadSealed(context.Context, string) (int64, error) { return 123, nil }
func (f *hgFakeSealed) HGDelete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}
func (f *hgFakeSealed) HGSignRead(context.Context, string) (string, error) {
	return "https://download.example.test/object", nil
}

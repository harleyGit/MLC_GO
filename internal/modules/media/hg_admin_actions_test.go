package media

import (
	HGRouter "MLC_GO/internal/pkg/hg_router"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	Permission "MLC_GO/internal/pkg/permission"
	Utils "MLC_GO/internal/pkg/utils"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

const hgAdminActionAsset = "019ac000-0000-7000-8000-000000000001"

type hgAdminReadStub struct {
	key string        // 签名地址对应的封存键；为空表示没有调用签名。
	ttl time.Duration // 签名有效期，仅在确实签名后写入。
}

func (s *hgAdminReadStub) hgSignRead(_ context.Context, key string, ttl time.Duration) (string, error) {
	s.key, s.ttl = key, ttl
	return "https://store.example.invalid/preview", nil
}

func TestHGMediaAdminActionsProtectedRoutes(t *testing.T) {
	full := HGRouteCatalog()
	for _, tc := range []struct {
		path string                  // route catalog 中的完整路径。
		verb string                  // 指定唯一 HTTP 方法。
		perm Permission.HGPermission // 独立于盘点读取的操作权限。
	}{
		{hgBasePath + "/admin/assets/preview", http.MethodGet, Permission.HGMediaAssetPreview},
		{hgBasePath + "/admin/assets/delete", http.MethodPost, Permission.HGMediaAssetDelete},
	} {
		t.Run(tc.path, func(t *testing.T) {
			match := slices.IndexFunc(full, func(item HGRouter.RouteCatalogItem) bool { return item.Path == tc.path })
			if match < 0 || full[match].Method != tc.verb || !full[match].NeedAuth || full[match].PermissionMatch != HGRouter.HGPermissionMatchAll ||
				!slices.Equal(full[match].Permissions, []Permission.HGPermission{Permission.HGMediaInventoryRead, tc.perm}) {
				t.Fatalf("管理操作缺少独立 IAM 权限: %v", full)
			}
			root := http.NewServeMux()
			root.Handle(hgBasePath+"/", http.StripPrefix(hgBasePath, (&HGModule{handler: HGNewHandler(nil)}).Handler()))
			res := httptest.NewRecorder()
			root.ServeHTTP(res, httptest.NewRequest(tc.verb, tc.path, nil))
			if res.Code != http.StatusBadRequest {
				t.Fatalf("无公共鉴权头应由 Guard 返回 400，结果 %d", res.Code)
			}
			root = http.NewServeMux()
			root.Handle(hgBasePath+"/", http.StripPrefix(hgBasePath, (&HGInventoryModule{handler: HGNewHandler(nil)}).Handler()))
			res = httptest.NewRecorder()
			root.ServeHTTP(res, httptest.NewRequest(tc.verb, tc.path, nil))
			if res.Code != http.StatusNotFound {
				t.Fatalf("无对象存储时不得挂载操作，结果 %d", res.Code)
			}
		})
	}
}

func TestHGMediaAdminPreviewPurposeOwnershipAndReference(t *testing.T) {
	sealed := "sealed/" + strings.ReplaceAll(hgAdminActionAsset, "-", "")
	for _, tc := range []struct {
		name, purpose, mime, actor, session, bucket, sealed string
		referenced                                          bool
		wantTTL                                             time.Duration
	}{
		{"低敏图", "product_main", "image/jpeg", "admin", "attached", "mlc-temp", sealed, true, 30 * time.Second},
		{"本人草稿视频", "video_source", "video/mp4", "owner", "ready", "mlc-temp", sealed, false, 5 * time.Minute},
		{"本人草稿音频", "chat_audio", "audio/ogg", "owner", "ready", "mlc-temp", sealed, false, 5 * time.Minute},
		{"他人私有音频", "chat_audio", "audio/ogg", "admin", "ready", "mlc-temp", sealed, false, 0},
		{"已绑定私有音频", "chat_audio", "audio/ogg", "owner", "attached", "mlc-temp", sealed, true, 0},
		{"已存在业务引用", "video_source", "video/mp4", "owner", "ready", "mlc-temp", sealed, true, 0},
		{"身份凭证", "user_identity", "image/jpeg", "owner", "ready", "mlc-temp", sealed, false, 0},
		{"错误封存存储", "product_main", "image/jpeg", "admin", "ready", "other-bucket", sealed, false, 0},
		{"错误封存对象", "product_main", "image/jpeg", "admin", "ready", "mlc-temp", "sealed/00000000000000000000000000000000", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := hgMockDB(t)
			signer := &hgAdminReadStub{}
			service := &HGService{db: db, bucket: "mlc-temp", previewRead: signer}
			mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaAdminPreviewSQL)).WithArgs(hgAdminActionAsset).
				WillReturnRows(sqlmock.NewRows([]string{"owner", "purpose", "mime", "status", "bucket", "sealed", "session", "referenced"}).
					AddRow("owner", tc.purpose, tc.mime, "ready", tc.bucket, tc.sealed, tc.session, tc.referenced))
			result, err := service.hgPreviewAdminAsset(context.Background(), tc.actor, hgAdminActionAsset)
			if tc.wantTTL == 0 {
				if !errors.Is(err, HGErrNotFound) || result.URL != "" || signer.key != "" {
					t.Fatalf("拒绝场景不得签名，结果=%+v，错误=%v", result, err)
				}
			} else if err != nil || result.URL == "" || result.MIME != tc.mime ||
				signer.key != sealed || signer.ttl != tc.wantTTL || result.ExpiresIn != int(tc.wantTTL.Seconds()) {
				t.Fatalf("签名结果或有效期不符，结果=%+v，错误=%v", result, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGMediaAdminDeleteQueuesAndAuditsOrLeavesStateUnchanged(t *testing.T) {
	key := "quarantine/" + strings.ReplaceAll(hgAdminActionAsset, "-", "")
	sealed := "sealed/" + strings.ReplaceAll(hgAdminActionAsset, "-", "")
	for _, tc := range []struct {
		name, status, purpose, actor string
		protected, referenced        bool
		want                         error
	}{
		{"入队", "ready", "product_main", "admin", false, false, nil},
		{"本人草稿音频", "ready", "chat_audio", "owner", false, false, nil},
		{"重复请求", "deleting", "product_main", "admin", false, false, nil},
		{"仍有引用", "ready", "product_main", "admin", false, true, HGErrConflict},
		{"解绑保护期内", "ready", "product_main", "admin", true, false, HGErrConflict},
		{"审核中", "processing", "product_main", "admin", false, false, HGErrConflict},
		{"他人的私有音频", "ready", "chat_audio", "admin", false, false, HGErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := hgMockDB(t)
			service := &HGService{db: db, bucket: "mlc-temp", sealed: &hgFakeSealed{}}
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaAdminDeleteSessionSQL)).WithArgs(hgAdminActionAsset).
				WillReturnRows(sqlmock.NewRows([]string{"id", "owner", "purpose", "key", "status", "protected"}).
					AddRow("upload", "owner", tc.purpose, key, tc.status, tc.protected))
			mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaAdminDeleteAssetSQL)).WithArgs(hgAdminActionAsset).
				WillReturnRows(sqlmock.NewRows([]string{"owner", "purpose", "key", "bucket", "status", "sealed"}).
					AddRow("owner", tc.purpose, key, "mlc-temp", tc.status, sealed))
			switch {
			case tc.referenced:
				mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaHasRefSQL)).WithArgs(hgAdminActionAsset).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectRollback()
			case tc.want != nil || tc.status == "deleting":
				mock.ExpectRollback()
			default:
				mock.ExpectQuery(regexp.QuoteMeta(SQL.HGMediaHasRefSQL)).WithArgs(hgAdminActionAsset).WillReturnError(sql.ErrNoRows)
				mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaExpireAssetSQL)).WithArgs(hgAdminActionAsset).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaExpireSessionSQL)).WithArgs("upload").WillReturnResult(sqlmock.NewResult(0, 1))
				for _, object := range []string{key, sealed} {
					mock.ExpectExec(regexp.QuoteMeta(SQL.HGMediaAdminDeleteQueueSQL)).
						WithArgs(hgAdminActionAsset, "mlc-temp", object).WillReturnResult(sqlmock.NewResult(1, 1))
				}
				mock.ExpectExec(regexp.QuoteMeta(SQL.HGInsertIAMAudit)).
					WithArgs(tc.actor, "media.asset.delete", "media_asset", hgAdminActionAsset, "test-request", `{"status":"deleting"}`).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			}
			result, err := service.hgDeleteAdminAsset(Utils.InjectTID(context.Background(), "test-request"), tc.actor, hgAdminActionAsset)
			if !errors.Is(err, tc.want) || tc.want == nil && (result.AssetID != hgAdminActionAsset || result.Status != "deleting") {
				t.Fatalf("删除结果=%+v，错误=%v，期望错误=%v", result, err, tc.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGMediaAdminRejectsLegacyIDsBeforeStorageLookup(t *testing.T) {
	service := &HGService{}
	for _, id := range []string{"123", strings.ReplaceAll(hgAdminActionAsset, "-", ""), "00000000-0000-0000-0000-000000000000"} {
		if _, err := service.hgDeleteAdminAsset(context.Background(), "admin", id); !errors.Is(err, HGErrInvalid) {
			t.Fatalf("旧 ID 未在入口拒绝：%v", err)
		}
		if _, err := service.hgPreviewAdminAsset(context.Background(), "admin", id); !errors.Is(err, HGErrInvalid) {
			t.Fatalf("旧 ID 未在入口拒绝：%v", err)
		}
	}
}

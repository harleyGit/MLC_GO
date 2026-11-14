package media

import (
	Permission "MLC_GO/internal/pkg/permission"
	"context"
	"errors"
	"net/http"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGInventoryRouteHasIndependentReadPermission(t *testing.T) {
	for _, route := range HGRouteCatalog() {
		if route.Path != "/api/v1/media/admin/assets/list" {
			continue
		}
		if route.Method != http.MethodGet || !route.NeedAuth || len(route.Permissions) != 1 || route.Permissions[0] != Permission.HGMediaInventoryRead {
			t.Fatalf("盘点路由未限制活动管理员和专用权限: %+v", route)
		}
		return
	}
	t.Fatal("媒体盘点路由未出现在 API Guard 同源目录")
}

func TestHGInventoryCursorRejectsInvalidAndCrossKind(t *testing.T) {
	db, mock := hgMockDB(t)
	service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
	for _, tc := range []struct{ kind, cursor string }{
		{"invalid", ""},
		{"image", "video:1:123"},
		{"", ":5:123"},
		{"image", "image:2:123"},
		{"", ":4:bad-hex"},
		{"", ":1:1 OR 1=1"},
		{"", ":0:invalid-id"},
	} {
		if _, err := service.HGInventory(context.Background(), tc.kind, tc.cursor, 20); !errors.Is(err, HGErrInvalid) {
			t.Fatalf("非法游标仍访问 SQL: %+v, %v", tc, err)
		}
	}
	if _, err := service.HGInventory(context.Background(), "", "", 51); !errors.Is(err, HGErrInvalid) {
		t.Fatal("超过页上限仍放行")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGInventorySourceCursorAndCategory(t *testing.T) {
	db, mock := hgMockDB(t)
	service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT asset_id,purpose,mime_type,file_size,status FROM media_assets WHERE media_kind=? ORDER BY asset_id DESC LIMIT ?")).
		WithArgs("image", 3).WillReturnRows(sqlmock.NewRows([]string{"asset_id", "purpose", "mime_type", "file_size", "status"}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,status FROM mall_product_image ORDER BY id DESC LIMIT ?")).
		WithArgs(3).WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(uint64(9), 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM users WHERE hg_has_avatar=1 ORDER BY id DESC LIMIT ?")).
		WithArgs(2).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uint64(8)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT HEX(id),media_type,mime_type,byte_size,state FROM im_private_attachment WHERE media_type=? ORDER BY id DESC LIMIT ?")).
		WithArgs("image", 1).WillReturnRows(sqlmock.NewRows([]string{"id", "media_type", "mime_type", "byte_size", "state"}).AddRow("0123456789ABCDEF0123456789ABCDEF", "image", "image/png", uint64(100), "ready"))
	page, err := service.HGInventory(context.Background(), "image", "", 2)
	if err != nil || len(page.Items) != 2 || page.Items[0].Source != "legacy_mall" || page.Items[1].Source != "legacy_avatar" || page.NextCursor != "image:3:8" {
		t.Fatalf("跨来源分页或类型过滤不正确: %+v, %v", page, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM users WHERE hg_has_avatar=1 AND id<? ORDER BY id DESC LIMIT ?")).
		WithArgs(uint64(8), 3).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT HEX(id),media_type,mime_type,byte_size,state FROM im_private_attachment WHERE media_type=? ORDER BY id DESC LIMIT ?")).
		WithArgs("image", 3).WillReturnRows(sqlmock.NewRows([]string{"id", "media_type", "mime_type", "byte_size", "state"}).AddRow("0123456789ABCDEF0123456789ABCDEF", "image", "image/png", uint64(100), "ready"))
	next, err := service.HGInventory(context.Background(), "image", page.NextCursor, 2)
	if err != nil || len(next.Items) != 1 || next.NextCursor != "" || next.Items[0].Status != "legacy:ready" || next.Items[0].Purpose != "legacy_im" {
		t.Fatalf("旧 IM 私有附件元数据游标错误: %+v, %v", next, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGInventoryStopsOnMissingLegacySource(t *testing.T) {
	db, mock := hgMockDB(t)
	service, _ := HGNewService(db, &hgFakeStore{}, "mlc-temp")
	mock.ExpectQuery("FROM media_assets ORDER BY asset_id DESC LIMIT").WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"asset_id", "purpose", "mime_type", "file_size", "status"}))
	mock.ExpectQuery("FROM mall_product_image ORDER BY id DESC LIMIT").WithArgs(2).WillReturnError(errors.New("缺少旧来源表"))
	page, err := service.HGInventory(context.Background(), "", "", 1)
	if err == nil || len(page.Items) != 0 {
		t.Fatalf("旧来源失败仍返回局部结果: %+v, %v", page, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

package commerce

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func hgTestDraft() HGDraftInput {
	return HGDraftInput{CategoryID: "1", ProductType: "PHYSICAL", Title: "测试实物", ImageURLs: []string{"/commerce/one.jpg"}, SKUs: []HGSKUInput{{SKUCode: "BLACK", Specs: map[string]string{"颜色": "黑色"}, PriceCent: 100, InitialStock: 0}, {SKUCode: "WHITE", Specs: map[string]string{"颜色": "白色"}, PriceCent: 101, InitialStock: 1}}}
}

// TestHGAdminCategoriesRequiresReviewButNotMerchant 验证管理员可读平台类目，商家接口仍需要已批准商家身份。
func TestHGAdminCategoriesRequiresReviewButNotMerchant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := HGNewService(HGNewRepository(db), nil)
	if _, err := service.HGAdminCategories(context.Background(), "admin"); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("无审核权限必须拒绝: %v", err)
	}
	service.reviewer = hgCommerceTestReviewer{}
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceCategorySQL)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "parent_id", "depth"}).AddRow(12, "普通叶子类目", nil, 1))
	items, err := service.HGAdminCategories(context.Background(), "admin")
	if err != nil || len(items) != 1 || items[0].ID != "12" {
		t.Fatalf("管理员类目读取失败: %v %+v", err, items)
	}
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceMerchantSQL)).WithArgs("admin").WillReturnRows(sqlmock.NewRows([]string{"merchant_id"}))
	if _, err := service.HGCategories(context.Background(), "admin"); !errors.Is(err, ErrHGMerchantRequired) {
		t.Fatalf("未入驻管理员不应获商家身份: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGMerchantCategoryDenialExplainsRequirement(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/commerce/merchant/categories", nil)
	hgError(w, r, ErrHGMerchantRequired)
	var body struct {
		Code int `json:"code"` // 权限失败统一使用 ForbiddenCode，不再冒充参数无效。
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "mall_merchant.status=1") || !strings.Contains(w.Body.String(), "mall:merchant:review") || body.Code != 300004 {
		t.Fatalf("商家身份拒绝响应未说明条件或管理员路径: %d %s", w.Code, w.Body.String())
	}
}

func TestHGValidateDraftBoundaries(t *testing.T) {
	base := hgTestDraft()
	if err := hgValidateDraft(base, false); err != nil {
		t.Fatalf("有效人民币草稿被拒绝：%v", err)
	}
	cases := []struct {
		name string
		edit func(*HGDraftInput)
		want error
	}{
		{"旧商品币种不可混用", func(d *HGDraftInput) { d.ProductType = "COIN" }, ErrHGInvalid},
		{"零价禁止", func(d *HGDraftInput) { d.SKUs[0].PriceCent = 0 }, ErrHGInvalid},
		{"重复规格拒绝", func(d *HGDraftInput) { d.SKUs[1].Specs = map[string]string{"颜色": "黑色"} }, ErrHGConflict},
		{"遗漏规格组合拒绝", func(d *HGDraftInput) {
			d.SKUs[0].Specs = map[string]string{"颜色": "黑色", "尺码": "S"}
			d.SKUs[1].Specs = map[string]string{"颜色": "白色", "尺码": "M"}
		}, ErrHGInvalid},
		{"图片 URL 不重复", func(d *HGDraftInput) { d.ImageURLs = append(d.ImageURLs, d.ImageURLs[0]) }, ErrHGInvalid},
		{"编辑需乐观锁版本", func(d *HGDraftInput) {}, ErrHGInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := hgTestDraft()
			tc.edit(&d)
			editing := tc.name == "编辑需乐观锁版本"
			err := hgValidateDraft(d, editing)
			if !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
				t.Fatalf("草稿校验=%v，期望=%v", err, tc.want)
			}
		})
	}
}

func TestHGReviewRechecksMerchantAndCategory(t *testing.T) {
	for _, tc := range []struct {
		name     string
		owner    string
		approve  bool
		category bool
		want     error
	}{
		{"本人不可自审", "reviewer", true, false, ErrHGForbidden},
		{"已禁用类目不可审核通过", "merchant-user", true, false, ErrHGInvalid},
		{"审核通过推进公开版本", "merchant-user", true, true, nil},
		{"驳回不改变公开版本", "merchant-user", false, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceReviewLockSQL)).WithArgs("CPRD_1").WillReturnRows(sqlmock.NewRows([]string{"audit_status", "sale_status", "version", "draft_version", "published_version", "user_id", "category_id"}).AddRow("PENDING", "ON_SHELF", 3, 2, 1, tc.owner, 5))
			if tc.approve && tc.owner != "reviewer" {
				rows := sqlmock.NewRows([]string{"id"})
				if tc.category {
					rows.AddRow(5)
				}
				mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceCategoryLeafSQL)).WithArgs(uint64(5)).WillReturnRows(rows)
			}
			if tc.want == nil {
				reason := ""
				status := "APPROVED"
				published := 2
				if !tc.approve {
					reason = "图片不清晰"
					status = "REJECTED"
					published = 1
				} else {
					mock.ExpectExec(regexp.QuoteMeta(q.HGCommerceInventoryEnsureSQL)).WithArgs("CPRD_1", uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
				}
				mock.ExpectExec(regexp.QuoteMeta(q.HGCommerceReviewVersionSQL)).WithArgs("reviewer", reason, "CPRD_1", uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta(q.HGCommerceProductStateSQL)).WithArgs(status, "ON_SHELF", uint64(2), published, "CPRD_1", uint64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			reviewReason := "图片不清晰"
			if tc.approve {
				reviewReason = ""
			}
			err = HGNewRepository(db).HGReview(context.Background(), "reviewer", "CPRD_1", tc.approve, reviewReason)
			if !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
				t.Fatalf("审核结果=%v，期望=%v", err, tc.want)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGSubmitRejectsReviewedVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceOwnerLockSQL)).WithArgs("CPRD_1", "merchant-user").WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "category_id", "audit_status", "sale_status", "version", "draft_version", "published_version"}).AddRow("SHOP_1", 5, "REJECTED", "OFF_SHELF", 3, 1, 0))
	mock.ExpectRollback()
	if err = HGNewRepository(db).HGSubmit(context.Background(), "merchant-user", "CPRD_1", 3); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("已驳回版本必须编辑后重审：%v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGSubmitReplaySameOwnerAndVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceOwnerLockSQL)).WithArgs("CPRD_1", "owner").WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "category_id", "audit_status", "sale_status", "version", "draft_version", "published_version"}).AddRow("SHOP_1", 5, "PENDING", "OFF_SHELF", 4, 1, 0))
	mock.ExpectRollback()
	if err = HGNewRepository(db).HGSubmit(context.Background(), "owner", "CPRD_1", 3); err != nil {
		t.Fatalf("本人同版本提交重试应返回首次成功：%v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGPublicDetailOnlyReadsPublishedVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommercePublicOneSQL)).WithArgs("CPRD_1").WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "merchant_id", "category_id", "audit_status", "sale_status", "version", "draft_version", "published_version"}).AddRow(1, "CPRD_1", "SHOP_1", 5, "PENDING", "ON_SHELF", 4, 2, 1))
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceVersionSQL)).WithArgs("CPRD_1", uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"title", "subtitle", "description", "images_json", "reject_reason"}).AddRow("已审核标题", "", "公开介绍", `["/one.jpg"]`, ""))
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceSKUsSQL)).WithArgs("CPRD_1", uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"sku_id", "sku_code", "specs_json", "price_cent", "initial_stock"}).AddRow("CSKU_1", "BLACK", `{}`, 100, 2))
	p, err := HGNewRepository(db).HGDetail(context.Background(), "viewer", "CPRD_1", "public")
	if err != nil || p.Title != "已审核标题" || p.AuditStatus != "APPROVED" || p.DraftVersion != 1 || p.PublishedVersion != 1 {
		t.Fatalf("公开视图泄露草稿或读取失败：%+v %v", p, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGListOwnerScopeBindsUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceMyPageSQL)).WithArgs("owner", uint64(99)).WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "merchant_id", "category_id", "audit_status", "sale_status", "version", "draft_version", "published_version", "title", "images_json", "price"}))
	page, err := HGNewRepository(db).HGList(context.Background(), "owner", 99, "mine")
	if err != nil || len(page.List) != 0 {
		t.Fatalf("本人列表查询失败：%+v %v", page, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGCreateDraftRejectsReusedKeyWithDifferentBody(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	draft := hgTestDraft()
	other := hgTestDraft()
	other.Title = "原始内容"
	old, _ := json.Marshal(other)
	oldHash := sha256.Sum256(old)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceMerchantLockSQL)).WithArgs("merchant-user").WillReturnRows(sqlmock.NewRows([]string{"merchant_id"}).AddRow("SHOP_1"))
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceCreateReplaySQL)).WithArgs("SHOP_1", "stable-key").WillReturnRows(sqlmock.NewRows([]string{"request_hash", "product_id"}).AddRow(oldHash[:], "CPRD_1"))
	mock.ExpectRollback()
	_, err = HGNewRepository(db).HGCreateDraft(context.Background(), "merchant-user", "stable-key", draft)
	if !errors.Is(err, ErrHGConflict) {
		t.Fatalf("同幂等键异正文应拒绝：%v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGArchivePreservesHistoryAndRejectsPending(t *testing.T) {
	for _, tc := range []struct {
		name, audit string
		want        error
	}{
		{"待审不得归档", "PENDING", ErrHGConflict},
		{"已上架归档", "APPROVED", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceOwnerLockSQL)).WithArgs("CPRD_1", "owner").WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "category_id", "audit_status", "sale_status", "version", "draft_version", "published_version"}).AddRow("SHOP_1", 5, tc.audit, "ON_SHELF", 2, 1, 1))
			if tc.want == nil {
				mock.ExpectExec(regexp.QuoteMeta(q.HGCommerceProductStateSQL)).WithArgs("APPROVED", "DELETED", uint64(1), uint64(1), "CPRD_1", uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err = HGNewRepository(db).HGArchive(context.Background(), "owner", "CPRD_1", 2)
			if !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
				t.Fatalf("归档结果=%v，期望=%v", err, tc.want)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

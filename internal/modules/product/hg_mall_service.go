package product

import (
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	ErrHGInvalid   = errors.New("商城参数非法")
	ErrHGConflict  = errors.New("申请或审核状态冲突")
	ErrHGForbidden = errors.New("无商家或审核权限")
)

// HGReviewer 复用 Ops 数据库 RBAC 的用户权限，不接受客户端自报的管理身份。
type HGReviewer interface {
	HasAssetPermission(context.Context, string, string) (bool, error)
}

// HGService 将目录、商家准入和审核规则置于 HTTP 与数据库之间。
type HGService struct {
	store    HGStore      // 商家准入和商品目录的 MySQL 读写接口。
	reviewer HGReviewer   // 服务端 Ops RBAC 审核鉴权，不能使用前端角色声明。
	orders   HGOrderStore // 同库实物订单/coin/库存短事务接口；由模块构造时注入。
}

func HGNewService(store HGStore, reviewer HGReviewer) *HGService {
	return &HGService{store: store, reviewer: reviewer}
}

// HGWithOrders 在模块装配时注入与商品目录共用 MySQL 池的订单仓储。
func (s *HGService) HGWithOrders(orders HGOrderStore) *HGService {
	s.orders = orders
	return s
}

var hgCategories = map[string]bool{"digital": true, "appliance": true, "clothing": true, "beauty": true, "membership": true}

func hgText(value string, max int) bool {
	return utf8.ValidString(value) && strings.TrimSpace(value) == value && value != "" && utf8.RuneCountInString(value) <= max && !strings.ContainsAny(value, "<>\x00")
}

func hgCursor(text string) (uint64, error) {
	if text == "" {
		return 0, nil
	}
	if len(text) > 20 {
		return 0, ErrHGInvalid
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return 0, ErrHGInvalid
		}
	}
	id, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, ErrHGInvalid
	}
	return id, nil
}

func (s *HGService) hgApprove(ctx context.Context, user string) error {
	if user == "" || s.reviewer == nil {
		return ErrHGForbidden
	}
	allowed, err := s.reviewer.HasAssetPermission(ctx, user, "mall.merchant.review")
	if err != nil {
		return err
	}
	if !allowed {
		return ErrHGForbidden
	}
	return nil
}

// Apply creates one pending shop per user; rejected applications may be resubmitted in place.
func (s *HGService) Apply(ctx context.Context, user, shopName string) (HGMerchantApplication, error) {
	if user == "" || !hgText(shopName, 128) {
		return HGMerchantApplication{}, ErrHGInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	m := HGMerchantApplication{MerchantID: UtilsPackage.GenerateBusinessID("MER"), UserID: user, ShopName: shopName}
	if err := s.store.Apply(ctx, m); err != nil {
		return HGMerchantApplication{}, err
	}
	return s.store.Merchant(ctx, user)
}

func (s *HGService) Merchant(ctx context.Context, user string) (HGMerchantApplication, error) {
	if user == "" {
		return HGMerchantApplication{}, ErrHGForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	return s.store.Merchant(ctx, user)
}

func (s *HGService) PendingMerchants(ctx context.Context, user, cursor string) (HGApplicationPage, error) {
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	if err := s.hgApprove(ctx, user); err != nil {
		return HGApplicationPage{}, err
	}
	id, err := hgCursor(cursor)
	if err != nil {
		return HGApplicationPage{}, err
	}
	items, err := s.store.PendingMerchants(ctx, id)
	if err != nil {
		return HGApplicationPage{}, err
	}
	page := HGApplicationPage{List: items, HasMore: len(items) > 20}
	if page.HasMore {
		page.List = items[:20]
		page.NextCursor = strconv.FormatUint(page.List[19].ID, 10)
	}
	// 数据库拒绝审核人审核自己的申请；按原始页计算游标后再过滤，避免跳过其他申请。
	reviewable := make([]HGMerchantApplication, 0, len(page.List))
	for _, item := range page.List {
		if item.UserID != user {
			reviewable = append(reviewable, item)
		}
	}
	page.List = reviewable
	return page, nil
}

func (s *HGService) ReviewMerchant(ctx context.Context, user, merchantID string, approve bool) error {
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	if err := s.hgApprove(ctx, user); err != nil {
		return err
	}
	if !hgBusinessID(merchantID) {
		return ErrHGInvalid
	}
	status := int8(2)
	if approve {
		status = 1
	}
	return s.store.ReviewMerchant(ctx, user, merchantID, status)
}

func hgBusinessID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

// Publish stores a single-SKU M币 catalog draft; this never reserves stock, charges coins or grants benefits.
func (s *HGService) Publish(ctx context.Context, user string, draft HGProductDraft) (HGCatalogItem, error) {
	if user == "" || !hgText(draft.Title, 128) || !hgText(draft.Summary, 512) || !hgText(draft.SKUTitle, 128) || draft.Amount < 1 || draft.Amount > 9007199254740991 || !hgCategories[draft.Category] ||
		(draft.Type == "MEMBERSHIP" && (draft.Category != "membership" || draft.Stock != 0)) || (draft.Type == "PHYSICAL_PRODUCT" && (draft.Category == "membership" || draft.Stock == 0 || draft.Stock > 100000 || draft.Amount > int64(CoinRepositoryPackage.HGMaxMallMutationAmount))) || (draft.Type != "MEMBERSHIP" && draft.Type != "PHYSICAL_PRODUCT") {
		return HGCatalogItem{}, ErrHGInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	m, err := s.store.Merchant(ctx, user)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && m.Status != 1) {
		return HGCatalogItem{}, ErrHGForbidden
	}
	if err != nil {
		return HGCatalogItem{}, err
	}
	item := HGCatalogItem{ProductID: UtilsPackage.GenerateBusinessID("PRD"), SKUID: UtilsPackage.GenerateBusinessID("PSKU"), MerchantID: m.MerchantID, ShopName: m.ShopName, Title: draft.Title, Type: draft.Type, Category: draft.Category, Summary: draft.Summary, SKUTitle: draft.SKUTitle, Amount: draft.Amount, Available: draft.Stock}
	if err := s.store.Publish(ctx, user, item); err != nil {
		return HGCatalogItem{}, err
	}
	return item, nil
}

func (s *HGService) ReviewProduct(ctx context.Context, user, productID string, approve bool) error {
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	if err := s.hgApprove(ctx, user); err != nil {
		return err
	}
	if !hgBusinessID(productID) {
		return ErrHGInvalid
	}
	status := int8(2)
	if approve {
		status = 1
	}
	return s.store.ReviewProduct(ctx, user, productID, status)
}

func (s *HGService) List(ctx context.Context, category, cursor string) (HGPage, error) {
	if category != "" && !hgCategories[category] {
		return HGPage{}, ErrHGInvalid
	}
	id, err := hgCursor(cursor)
	if err != nil {
		return HGPage{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	items, err := s.store.List(ctx, category, id)
	if err != nil {
		return HGPage{}, err
	}
	return hgProductPage(items), nil
}

func (s *HGService) Detail(ctx context.Context, id string) (HGCatalogItem, error) {
	if !hgBusinessID(id) {
		return HGCatalogItem{}, ErrHGInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	return s.store.Detail(ctx, id)
}

func (s *HGService) Mine(ctx context.Context, user, cursor string) (HGPage, error) {
	if user == "" {
		return HGPage{}, ErrHGForbidden
	}
	id, err := hgCursor(cursor)
	if err != nil {
		return HGPage{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	items, err := s.store.Mine(ctx, user, id)
	if err != nil {
		return HGPage{}, err
	}
	return hgProductPage(items), nil
}

func (s *HGService) PendingProducts(ctx context.Context, user, cursor string) (HGPage, error) {
	ctx, cancel := context.WithTimeout(ctx, HGTimeLimit)
	defer cancel()
	if err := s.hgApprove(ctx, user); err != nil {
		return HGPage{}, err
	}
	id, err := hgCursor(cursor)
	if err != nil {
		return HGPage{}, err
	}
	items, err := s.store.PendingProducts(ctx, id)
	if err != nil {
		return HGPage{}, err
	}
	return hgProductPage(items), nil
}

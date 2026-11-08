// Package commerce 提供独立的生产 CNY 商品发布及审核；交易接口未接通时商品只可浏览。
package commerce

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	ErrHGInvalid          = errors.New("商品参数无效")
	ErrHGForbidden        = errors.New("无商品操作权限")
	ErrHGMerchantRequired = errors.New("需要已批准的商家身份")
	ErrHGConflict         = errors.New("商品版本或状态冲突")
)

// HGCategory 是服务端管理的普通叶子类目；受监管类目首期不返回给商家。
type HGCategory struct {
	ID    string `json:"categoryId"` // 类目 BIGINT ID 的字符串表示，不经过 JS number。
	Name  string `json:"name"`       // 平台类目名称。
	Depth uint8  `json:"depth"`      // 所在层级，1-3。
}

// HGCategoryRequest 是平台管理员创建类目的请求；受监管类目创建后仍不可发布。
type HGCategoryRequest struct {
	ParentID  string `json:"parentId"`  // 上级 ID；空表示一级类目。
	Name      string `json:"name"`      // 类目名称，1-64字符。
	Regulated bool   `json:"regulated"` // 是否需资质；true 首期禁止销售。
}

// HGSKUInput 是商家草稿 SKU；初始库存只记录建议值，不代表可售库存或支付许可。
type HGSKUInput struct {
	SKUCode      string            `json:"skuCode"`      // 同商品同版本唯一编码，1-100位 ASCII。
	Specs        map[string]string `json:"specs"`        // 规格维度名和值，最多3维；同款组合不可重复。
	PriceCent    int64             `json:"priceCent"`    // 人民币分，1-1000000000。
	InitialStock int64             `json:"initialStock"` // 初始数量建议值，0-100000件；当前不用于下单。
}

// HGDraftInput 是生产 CNY 商品内容请求；服务端从登录态获取商家身份，不接收商家 ID 或应付金额。
type HGDraftInput struct {
	CategoryID      string       `json:"categoryId"`                // 可经营的叶子类目 BIGINT 字符串。
	ProductType     string       `json:"productType"`               // 首期仅 PHYSICAL。
	Title           string       `json:"title"`                     // 1-120字符纯文本。
	Subtitle        string       `json:"subtitle"`                  // 最多200字符纯文本，空表示没有副标题。
	Description     string       `json:"description"`               // 最多4096字符纯文本，空表示没有介绍。
	ImageURLs       []string     `json:"imageURLs"`                 // 1-5张本人上传的资产 URL；首张为主图。
	SKUs            []HGSKUInput `json:"skus"`                      // 1-200个规格组合。
	ExpectedVersion uint64       `json:"expectedVersion,omitempty"` // 编辑时必传的商品乐观锁版本；创建时为0。
}

// HGSKU 是生产人民币商品详情的销售规格快照；不返回库存可购买保证。
type HGSKU struct {
	HGSKUInput
	SKUID string `json:"skuId"` // 服务端生成的独立 CNY SKU 业务 ID。
}

// HGProduct 是生产 CNY 商品视图；公开详情只读取 publishedVersion，商家详情读取 draftVersion。
type HGProduct struct {
	ID               uint64   `json:"-"`                      // 内部列表游标。
	ProductID        string   `json:"productId"`              // 商品业务 ID。
	MerchantID       string   `json:"merchantId"`             // 店铺商家业务 ID。
	CategoryID       string   `json:"categoryId"`             // 可经营类目业务 ID 字符串。
	Currency         string   `json:"currency"`               // 始终为 CNY。
	ProductType      string   `json:"productType"`            // 首期始终为 PHYSICAL。
	AuditStatus      string   `json:"auditStatus"`            // DRAFT、PENDING、APPROVED、REJECTED。
	SaleStatus       string   `json:"saleStatus"`             // OFF_SHELF、ON_SHELF、DELETED。
	Version          uint64   `json:"version"`                // 写操作乐观锁版本。
	DraftVersion     uint64   `json:"draftVersion"`           // 本次待审核内容版本。
	PublishedVersion uint64   `json:"publishedVersion"`       // 对外已发布内容版本；0表示不可公开。
	Title            string   `json:"title"`                  // 当前可见版本的标题。
	Subtitle         string   `json:"subtitle"`               // 当前可见版本的副标题。
	Description      string   `json:"description"`            // 当前可见版本的纯文本介绍。
	ImageURLs        []string `json:"imageURLs"`              // 当前可见版本1-5张展示图。
	MinPriceCent     int64    `json:"minPriceCent"`           // 当前可见版本最低 SKU 单价，人民币分。
	RejectReason     string   `json:"rejectReason,omitempty"` // 本人草稿的审核驳回原因，公开列表不返回。
	SKUs             []HGSKU  `json:"skus,omitempty"`         // 详情最多200个 SKU；列表不加载。
}

// HGPage 是生产 CNY 商品有界游标页；不返回大表 count。
type HGPage struct {
	List       []HGProduct `json:"list"`       // 当前最多20个商品。
	NextCursor string      `json:"nextCursor"` // 最后一条内部 ID；无下一页为空。
	HasMore    bool        `json:"hasMore"`    // 是否还有后续记录。
}

// HGReviewer 复用现有 RBAC 审核权限；写事务仍验证审核人与商家不是同一用户。
type HGReviewer interface {
	HasAssetPermission(context.Context, string, string) (bool, error)
}

// HGService 编排校验与商品状态，Repository 负责短事务和归属二次校验。
type HGService struct {
	repo     *HGRepository // 生产商品 MySQL 仓储。
	reviewer HGReviewer    // 平台审核权限检查器。
}

// HGNewService 装配生产 CNY 商品领域，不访问旧商品或 debug 交易表。
func HGNewService(repo *HGRepository, reviewer HGReviewer) *HGService {
	return &HGService{repo: repo, reviewer: reviewer}
}

func hgText(text string, max int, required bool) bool {
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > max || (required && strings.TrimSpace(text) == "") || strings.TrimSpace(text) != text {
		return false
	}
	return !strings.ContainsAny(text, "<>\x00\r\n")
}

func hgID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func hgCategoryID(id string) (uint64, error) {
	if id == "" || len(id) > 20 || id[0] == '0' {
		return 0, ErrHGInvalid
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return 0, ErrHGInvalid
		}
	}
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil || n == 0 {
		return 0, ErrHGInvalid
	}
	return n, nil
}

func hgCursor(cursor string) (uint64, error) {
	if cursor == "" {
		return math.MaxUint64, nil
	}
	return hgCategoryID(cursor)
}

// hgValidateDraft 验证同版本 SKU 唯一、同维度及笛卡尔积完整性；不以客户端价格作为支付权威。
func hgValidateDraft(d HGDraftInput, editing bool) error {
	if _, err := hgCategoryID(d.CategoryID); err != nil {
		return err
	}
	if d.ProductType != "PHYSICAL" || !hgText(d.Title, 120, true) || !hgText(d.Subtitle, 200, false) || !hgText(d.Description, 4096, false) || len(d.ImageURLs) < 1 || len(d.ImageURLs) > 5 || len(d.SKUs) < 1 || len(d.SKUs) > 200 || (editing && d.ExpectedVersion == 0) || (!editing && d.ExpectedVersion != 0) {
		return ErrHGInvalid
	}
	images := make(map[string]bool, len(d.ImageURLs))
	for _, image := range d.ImageURLs {
		if image == "" || len(image) > 1024 || images[image] {
			return ErrHGInvalid
		}
		images[image] = true
	}
	codes := make(map[string]bool, len(d.SKUs))
	combos := make(map[[32]byte]bool, len(d.SKUs))
	dimensions := make(map[string]map[string]bool)
	for i, sku := range d.SKUs {
		if !hgID(sku.SKUCode) || len(sku.SKUCode) > 100 || codes[sku.SKUCode] || sku.PriceCent < 1 || sku.PriceCent > 1000000000 || sku.InitialStock < 0 || sku.InitialStock > 100000 || len(sku.Specs) > 3 {
			return ErrHGInvalid
		}
		codes[sku.SKUCode] = true
		if i == 0 {
			for name := range sku.Specs {
				dimensions[name] = make(map[string]bool)
			}
		}
		if len(sku.Specs) != len(dimensions) {
			return ErrHGInvalid
		}
		for name, value := range sku.Specs {
			if !hgText(name, 32, true) || !hgText(value, 64, true) {
				return ErrHGInvalid
			}
			if _, ok := dimensions[name]; !ok {
				return ErrHGInvalid
			}
			dimensions[name][value] = true
		}
		data, _ := json.Marshal(sku.Specs) // Go map 键规范化排序序列化，服务端计算规格哈希。
		hash := sha256.Sum256(data)
		if combos[hash] {
			return ErrHGConflict
		}
		combos[hash] = true
	}
	count := 1
	for _, values := range dimensions {
		count *= len(values)
	}
	if count != len(d.SKUs) {
		return ErrHGInvalid
	}
	return nil
}

// HGCategories 返回平台启用的普通叶子类目；无可经营类目时商家不能提交。
func (s *HGService) HGCategories(ctx context.Context, user string) ([]HGCategory, error) {
	if user == "" {
		return nil, ErrHGForbidden
	}
	return s.repo.HGCategories(ctx, user)
}

// HGAdminCategories 通过平台审核权限读取类目，不赋予商家身份或商品归属权限。
func (s *HGService) HGAdminCategories(ctx context.Context, user string) ([]HGCategory, error) {
	if err := s.hgReviewAllowed(ctx, user); err != nil {
		return nil, err
	}
	return s.repo.HGAdminCategories(ctx)
}

// HGCreateCategory 仅平台管理员可维护类目；受监管类目保留但不可发布。
func (s *HGService) HGCreateCategory(ctx context.Context, user string, request HGCategoryRequest) (HGCategory, error) {
	if err := s.hgReviewAllowed(ctx, user); err != nil {
		return HGCategory{}, err
	}
	if !hgText(request.Name, 64, true) {
		return HGCategory{}, ErrHGInvalid
	}
	return s.repo.HGCreateCategory(ctx, request)
}

func (s *HGService) hgReviewAllowed(ctx context.Context, user string) error {
	if user == "" || s.reviewer == nil {
		return ErrHGForbidden
	}
	ok, err := s.reviewer.HasAssetPermission(ctx, user, "mall.merchant.review")
	if err != nil {
		return err
	}
	if !ok {
		return ErrHGForbidden
	}
	return nil
}

// HGCreateDraft 创建可续编草稿，同商家相同 requestId 与正文重放只返回首次商品。
func (s *HGService) HGCreateDraft(ctx context.Context, user, requestID string, d HGDraftInput) (HGProduct, error) {
	if user == "" || !hgID(requestID) {
		return HGProduct{}, ErrHGInvalid
	}
	if err := hgValidateDraft(d, false); err != nil {
		return HGProduct{}, err
	}
	return s.repo.HGCreateDraft(ctx, user, requestID, d)
}

// HGEditDraft 创建新的审核内容版本，线上已发布版本继续供消费者浏览。
func (s *HGService) HGEditDraft(ctx context.Context, user, id string, d HGDraftInput) (HGProduct, error) {
	if user == "" || !hgID(id) {
		return HGProduct{}, ErrHGInvalid
	}
	if err := hgValidateDraft(d, true); err != nil {
		return HGProduct{}, err
	}
	return s.repo.HGEditDraft(ctx, user, id, d)
}

// HGSubmit 将草稿版本提交给平台；消费者仍只读取上次已审核版本。
func (s *HGService) HGSubmit(ctx context.Context, user, id string, version uint64) error {
	if user == "" || !hgID(id) || version == 0 {
		return ErrHGInvalid
	}
	return s.repo.HGSubmit(ctx, user, id, version)
}

// HGReview 审核人不得审核自己商品；驳回原因只在拒绝时必填。
func (s *HGService) HGReview(ctx context.Context, user, id string, approve bool, reason string) error {
	if err := s.hgReviewAllowed(ctx, user); err != nil {
		return err
	}
	if !hgID(id) || (!approve && !hgText(reason, 256, true)) || (approve && reason != "") {
		return ErrHGInvalid
	}
	return s.repo.HGReview(ctx, user, id, approve, reason)
}

// HGSetSale 商家只可上架已通过审核的商品；归档后不能重新上架。
func (s *HGService) HGSetSale(ctx context.Context, user, id string, version uint64, publish bool) error {
	if user == "" || !hgID(id) || version == 0 {
		return ErrHGInvalid
	}
	return s.repo.HGSetSale(ctx, user, id, version, publish)
}

// HGArchive 永久撤下本人商品，保留审核版本、SKU 与媒体以供历史审计；不删除任何订单事实。
func (s *HGService) HGArchive(ctx context.Context, user, id string, version uint64) error {
	if user == "" || !hgID(id) || version == 0 {
		return ErrHGInvalid
	}
	return s.repo.HGArchive(ctx, user, id, version)
}

// HGList 只读取已发布版本；没有可售库存与支付渠道时商品只供展示，不提供购买 API。
func (s *HGService) HGList(ctx context.Context, user, cursor string, scope string) (HGPage, error) {
	if user == "" {
		return HGPage{}, ErrHGForbidden
	}
	if scope != "public" && scope != "mine" && scope != "review" {
		return HGPage{}, ErrHGInvalid
	}
	if scope == "review" {
		if err := s.hgReviewAllowed(ctx, user); err != nil {
			return HGPage{}, err
		}
	}
	id, err := hgCursor(cursor)
	if err != nil {
		return HGPage{}, err
	}
	return s.repo.HGList(ctx, user, id, scope)
}

// HGDetail 按唯一业务 ID 定点查；公开视图不泄露草稿内容。
func (s *HGService) HGDetail(ctx context.Context, user, id string, scope string) (HGProduct, error) {
	if user == "" || !hgID(id) {
		return HGProduct{}, ErrHGInvalid
	}
	if scope != "mine" && scope != "public" && scope != "review" {
		return HGProduct{}, ErrHGInvalid
	}
	if scope == "review" {
		if err := s.hgReviewAllowed(ctx, user); err != nil {
			return HGProduct{}, err
		}
	}
	return s.repo.HGDetail(ctx, user, id, scope)
}

func hgExactlyOne(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrHGConflict
	}
	return nil
}

func hgPage(items []HGProduct) HGPage {
	page := HGPage{List: items, HasMore: len(items) > 20}
	if page.HasMore {
		page.List = items[:20]
		page.NextCursor = fmt.Sprint(page.List[19].ID)
	}
	return page
}

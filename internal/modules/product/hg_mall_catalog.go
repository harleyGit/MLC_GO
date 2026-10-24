package product

import "time"

// HGMerchantApplication 是商家准入审核视图；仅审核接口暴露申请者身份。
type HGMerchantApplication struct {
	ID         uint64 `json:"-"`                // 审核游标内部 ID，不作为公开协议字段。
	MerchantID string `json:"merchantId"`       // 商家业务标识。
	UserID     string `json:"userId,omitempty"` // 仅审核列表包含的申请者身份。
	ShopName   string `json:"shopName"`         // 店铺展示名称。
	Status     int8   `json:"status,omitempty"` // 审核状态：0待审核、1通过、2拒绝。
}

// HGCatalogItem 是 MLC 商城目录协议；amount 单位为 M币，实物商品库存是可用量的读取快照。
type HGCatalogItem struct {
	ID              uint64   `json:"-"`                         // 商品内部游标，只用于生成下一页位置。
	ProductID       string   `json:"productId"`                 // 商品对外业务标识。
	Type            string   `json:"type"`                      // PHYSICAL_PRODUCT 或 MEMBERSHIP，会员仅展示不履约。
	Title           string   `json:"title"`                     // 审核时商品标题。
	Category        string   `json:"category"`                  // 白名单类目编码。
	Summary         string   `json:"summary"`                   // 纯文本简介，不解析富文本。
	Description     string   `json:"description"`               // 纯文本商品介绍，最多4096字符；空表示旧商品未填写。
	ImageURLs       []string `json:"imageURLs"`                 // 最多5张已登记并绑定商品的图片 URL；旧商品返回空数组。
	MerchantID      string   `json:"merchantId"`                // 商家业务标识。
	ShopName        string   `json:"shopName"`                  // 店铺名称。
	SKUID           string   `json:"skuId"`                     // 单规格 SKU 业务标识。
	SKUTitle        string   `json:"skuTitle"`                  // 规格标题。
	Amount          int64    `json:"amount"`                    // 整数 M币单价，禁止客户端换算为真实现金价格。
	Available       uint32   `json:"available"`                 // 实物商品剩余可售件数；会员和旧目录记录为 0。
	Status          int8     `json:"status,omitempty"`          // 本人/审核列表的商城 review_status：0待审核、1通过、2拒绝；公开目录省略。
	LifecycleStatus int8     `json:"lifecycleStatus,omitempty"` // 本人商品状态：0未上架、1在售、2已归档；公开目录省略。
}

// HGPage 是商品游标页协议；nextCursor 为空表示无后续页，不返回大表总数。
type HGPage struct {
	List       []HGCatalogItem `json:"list"`       // 当前至多 20 个商品。
	NextCursor string          `json:"nextCursor"` // 下一页游标，空表示无下一页。
	HasMore    bool            `json:"hasMore"`    // 是否还有后续记录。
}

// HGApplicationPage 是待审商家游标页协议；仅有审核权限时返回。
type HGApplicationPage struct {
	List       []HGMerchantApplication `json:"list"`       // 至多 20 条待审申请。
	NextCursor string                  `json:"nextCursor"` // 下一页游标，空表示结束。
	HasMore    bool                    `json:"hasMore"`    // 是否还有待审申请。
}

// HGProductDraft 是发布请求协议；服务端生成 ID、校验类目和 M币金额，仅创建待审核草稿。
type HGProductDraft struct {
	Type        string   `json:"type"`        // PHYSICAL_PRODUCT 或 MEMBERSHIP。
	Category    string   `json:"category"`    // 有界白名单类目；会员类型仅能选会员。
	Title       string   `json:"title"`       // 1-128 个字符。
	Summary     string   `json:"summary"`     // 1-512 个字符纯文本。
	Description string   `json:"description"` // 0-4096 个字符的纯文本商品介绍。
	ImageURLs   []string `json:"imageURLs"`   // 至多5张当前商家本人已上传且未绑定的图片 URL。
	SKUTitle    string   `json:"skuTitle"`    // 1-128 个字符规格。
	Amount      int64    `json:"amount"`      // 1 到 2^53-1 的整数 M币。
	Stock       uint32   `json:"stock"`       // 实物首次发布的可售件数，1-100000；会员仅展示，固定为 0。
}

// HGTimeLimit 控制目录与审核单次数据库操作的最长时长。
const HGTimeLimit = 5 * time.Second

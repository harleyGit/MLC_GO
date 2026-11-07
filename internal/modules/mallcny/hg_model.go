package mallcny

import (
	"errors"
	"time"
)

var (
	ErrHGInvalid      = errors.New("人民币商城参数无效")
	ErrHGNotFound     = errors.New("人民币商城记录不存在")
	ErrHGConflict     = errors.New("人民币商城状态冲突")
	ErrHGQuoteChanged = errors.New("报价已变化，请重新确认")
	ErrHGUnavailable  = errors.New("模拟支付仅限已授权的隔离环境")
)

// HGProduct 为人民币商城公开详情协议；价格全部为人民币分，绝不读取 M币商品标价。
type HGProduct struct {
	ProductID     string `json:"productId"`     // 独立人民币商品业务 ID。
	MerchantID    string `json:"merchantId"`    // 商家业务 ID；只能由已审核商家创建。
	Title         string `json:"title"`         // 公开标题，纯文本。
	Description   string `json:"description"`   // 纯文本商品介绍；空为没有介绍。
	ImageURL      string `json:"imageUrl"`      // 公开展示图；空表示无图。
	SKUID         string `json:"skuId"`         // 当前唯一在售规格业务 ID。
	SKUTitle      string `json:"skuTitle"`      // 在售规格名称。
	PriceFen      int64  `json:"priceFen"`      // 单买价，人民币分。
	GroupPriceFen *int64 `json:"groupPriceFen"` // 拼单价，人民币分；nil 为不支持拼单。
	GroupMembers  uint8  `json:"groupMembers"`  // 成团目标人数，2-10；0 为不支持拼单。
	Available     uint32 `json:"available"`     // 可售库存快照，单位件。
	ShopName      string `json:"shopName"`      // 已审核商家店名。
}

// HGCreateProduct 为已审核商家的单规格商品发布协议，初始库存只在创建时写入。
type HGCreateProduct struct {
	Title         string `json:"title"`         // 1-128 字纯文本商品标题。
	Description   string `json:"description"`   // 最多4096字纯文本商品介绍。
	ImageURL      string `json:"imageUrl"`      // 最多1024字的已绑定图片 URL；空为无图。
	SKUTitle      string `json:"skuTitle"`      // 1-128 字规格名称。
	PriceFen      int64  `json:"priceFen"`      // 单买价，人民币分，1-10000000。
	GroupPriceFen *int64 `json:"groupPriceFen"` // 拼单价，人民币分；nil 为不启用。
	GroupMembers  uint8  `json:"groupMembers"`  // 0 或 2-10 人。
	Stock         uint32 `json:"stock"`         // 初始可售库存，1-100000 件。
}

// HGCoupon 为用户可领取券协议，券只适用单买与指定商品；拼单价不叠券。
type HGCoupon struct {
	CouponID     string    `json:"couponId"`     // 券模板 ID。
	ProductID    string    `json:"productId"`    // 适用商品 ID。
	ThresholdFen int64     `json:"thresholdFen"` // 使用门槛，人民币分。
	DiscountFen  int64     `json:"discountFen"`  // 减免金额，人民币分。
	Remaining    uint32    `json:"remaining"`    // 剩余可领券数快照。
	ExpiresAt    time.Time `json:"expiresAt"`    // 领券和下单截止时间，UTC。
}

// HGCreateCoupon 为商家创建优惠券模板协议；发券额度通过条件更新有界扣减。
type HGCreateCoupon struct {
	ProductID    string    `json:"productId"`    // 本店人民币商品 ID。
	ThresholdFen int64     `json:"thresholdFen"` // 单买使用门槛，人民币分。
	DiscountFen  int64     `json:"discountFen"`  // 单买抵扣金额，人民币分。
	Quantity     uint32    `json:"quantity"`     // 可领数量，1-100000。
	ExpiresAt    time.Time `json:"expiresAt"`    // 到期时间，UTC，必须晚于当前时间。
}

// HGOrderRequest 是单买、开团或参团的幂等下单协议；期望金额只用于避免静默涨价。
type HGOrderRequest struct {
	ProductID          string `json:"productId"`          // 人民币商品业务 ID。
	SKUID              string `json:"skuId"`              // 所选单规格 SKU ID。
	Quantity           uint8  `json:"quantity"`           // 购买件数，1-10。
	PurchaseType       string `json:"purchaseType"`       // single、group_create、group_join。
	GroupID            string `json:"groupId"`            // 参团 ID；其他方式必须为空。
	CouponID           string `json:"couponId"`           // 已领取券 ID；仅单买可传。
	Address            string `json:"address"`            // 1-512 字收货地址快照；不记录到 URL。
	ExpectedPayableFen int64  `json:"expectedPayableFen"` // 用户确认的应付金额，人民币分；创建时重算。
	RequestID          string `json:"requestId"`          // 1-64 位幂等标识，同用户同内容重试返回同订单。
}

// HGQuote 是服务端报价协议，金额为人民币分；预览不预占库存及券。
type HGQuote struct {
	UnitFen     int64 `json:"unitFen"`     // 当前单价，人民币分。
	TotalFen    int64 `json:"totalFen"`    // 单价乘数量，人民币分。
	DiscountFen int64 `json:"discountFen"` // 已领取券优惠，人民币分。
	PayableFen  int64 `json:"payableFen"`  // 应付金额，人民币分。
	Available   bool  `json:"available"`   // 报价时库存是否足够，非最终预留保证。
}

// HGOrder 是本人可见的人民币交易快照；status 0待付、1待发货、2待成团、3取消、4已退款。
type HGOrder struct {
	OrderID      string    `json:"orderId"`            // 订单业务 ID。
	ProductID    string    `json:"productId"`          // 商品业务 ID 快照。
	Title        string    `json:"title"`              // 下单时商品标题快照。
	Quantity     uint8     `json:"quantity"`           // 已购买件数。
	PurchaseType string    `json:"purchaseType"`       // single、group_create、group_join。
	GroupID      string    `json:"groupId,omitempty"`  // 拼团 ID；单买为空。
	CouponID     string    `json:"couponId,omitempty"` // 预占券 ID；不用券为空。
	TotalFen     int64     `json:"totalFen"`           // 商品小计，人民币分。
	DiscountFen  int64     `json:"discountFen"`        // 优惠金额，人民币分。
	PayableFen   int64     `json:"payableFen"`         // 应付金额，人民币分。
	Status       uint8     `json:"status"`             // 0待付、1待发货、2待成团、3取消、4已退款。
	ExpiresAt    time.Time `json:"expiresAt"`          // 待付款截止时间，UTC；支付后仍保留快照。
}

// HGGroup 是拼团公开进度协议；未支付用户不计入 paidMembers。
type HGGroup struct {
	GroupID         string    `json:"groupId"`         // 拼团业务 ID。
	ProductID       string    `json:"productId"`       // 关联商品 ID。
	TargetMembers   uint8     `json:"targetMembers"`   // 目标人数，2-10。
	PaidMembers     uint8     `json:"paidMembers"`     // 已模拟支付人数。
	ReservedMembers uint8     `json:"reservedMembers"` // 未支付订单占位人数。
	Status          uint8     `json:"status"`          // 0未激活、1招募中、2成团、3失败、4取消、5失败且退款结清。
	ExpiresAt       time.Time `json:"expiresAt"`       // 拼团截止时间，UTC。
}

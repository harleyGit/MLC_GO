package dto

// HGCreateRequest 是创建订单协议；金额、币数、展示名均不接受客户端传入。
type HGCreateRequest struct {
	HGSKUID     string `json:"skuId"`
	HGRequestID string `json:"requestId"`
}

// HGPayRequest 是严格的支付请求；paymentMethod 必须为 platform_debug、wechat 或 alipay。
type HGPayRequest struct {
	HGOrderID       string `json:"orderId"`
	HGPaymentMethod string `json:"paymentMethod"`
}

// HGBalance 是 coin 权威余额，单位为平台币，与人民币分没有固定换算比例。
// balance使用十进制字符串，与运维余额协议一致，避免JS数字超过53位后丢失精度。
type HGBalance struct {
	HGBalance string `json:"balance"`
}

// HGSKU 是用户可见档位；金额单位分，币数为整数，仅availableMethods中的渠道可支付。
type HGSKU struct {
	HGSKUID            string   `json:"skuId"`
	HGTitle            string   `json:"title"`
	HGDescription      string   `json:"description"`
	HGCurrency         string   `json:"currency"`
	HGPayAmount        uint64   `json:"payAmount"`
	HGCoinAmount       uint64   `json:"coinAmount"`
	HGBonusCoin        uint64   `json:"bonusCoin"`
	HGTotalCoin        uint64   `json:"totalCoin"`
	HGSupported        bool     `json:"supported"`
	HGPaymentAvailable bool     `json:"paymentAvailable"`
	HGPaymentMode      string   `json:"paymentMode"`
	HGAvailableMethods []string `json:"availableMethods"`
}

// HGSKUPage 使用十进制内部主键游标，默认20、最大100；空页也可能有下一游标。
// 为限制失效目录扫描，每次最多检查201个候选，客户端须以hasMore判断结束。
type HGSKUPage struct {
	HGList       []HGSKU `json:"list"`
	HGNextCursor string  `json:"nextCursor"`
	HGHasMore    bool    `json:"hasMore"`
}

// HGOrder 是仅订单所属用户可读的快照协议；时间为UTC RFC3339Nano，状态为pending/expired/paid。
type HGOrder struct {
	HGOrderID          string   `json:"orderId"`
	HGDisplayName      string   `json:"displayName"`
	HGTitle            string   `json:"title"`
	HGDescription      string   `json:"description"`
	HGPayAmount        uint64   `json:"payAmount"`
	HGTotalCoin        uint64   `json:"totalCoin"`
	HGStatus           string   `json:"status"`
	HGExpiresAt        string   `json:"expiresAt"`
	HGServerTime       string   `json:"serverTime"`
	HGPaymentAvailable bool     `json:"paymentAvailable"`
	HGPaymentMode      string   `json:"paymentMode"`
	HGAvailableMethods []string `json:"availableMethods"`
	HGPaidAt           string   `json:"paidAt,omitempty"`
	HGBalanceAfter     string   `json:"balanceAfter,omitempty"`
	HGCurrency         string   `json:"currency"`
}

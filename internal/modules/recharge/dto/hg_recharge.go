package dto

// HGRecord 是 GET /api/v1/recharge/records 的记录协议，金额不经浮点转换。
type HGRecord struct {
	HGOrderID     string `json:"orderId"`          // 充值订单业务标识。
	HGTitle       string `json:"title"`            // 档位标题快照。
	HGCurrency    string `json:"currency"`         // 支付币种。
	HGPayAmount   string `json:"payAmount"`        // 十进制整数分，避免客户端精度损失。
	HGTotalCoin   string `json:"totalCoin"`        // 十进制整数平台币数。
	HGStatus      string `json:"status"`           // pending/expired/paid/cancelled/processing/refunding/refunded。
	HGPaymentMode string `json:"paymentMode"`      // 创建时支付渠道快照。
	HGCreatedAt   string `json:"createdAt"`        // UTC RFC3339 时间。
	HGPaidAt      string `json:"paidAt,omitempty"` // UTC RFC3339 支付时间，未支付时省略。
}

// HGPage 是充值记录游标页，不提供总数。
type HGPage struct {
	HGList       []HGRecord `json:"list"`       // 最多 50 条，空页为 []。
	HGNextCursor string     `json:"nextCursor"` // 无下一页时为空字符串。
	HGHasMore    bool       `json:"hasMore"`    // 由 limit+1 判断。
}

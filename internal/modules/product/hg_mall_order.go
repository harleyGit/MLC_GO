package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

var (
	ErrHGStock = errors.New("库存不足或商品不可购买")
	ErrHGFunds = errors.New("M币余额不足或有效批次不足")
	ErrHGState = errors.New("订单状态不允许该操作")
)

// HGBuyRequest 是实物商品购买协议；requestId 对买家唯一且永久保存，重复请求必须保持商品、数量和地址完全一致。
type HGBuyRequest struct {
	ProductID       string `json:"productId"`       // 已审核实物商品业务标识。
	RequestID       string `json:"requestId"`       // 1-64 位 ASCII 幂等键，前端每次购买生成新值。
	Quantity        uint32 `json:"quantity"`        // 本期单 SKU，1-10 件。
	ReceiverName    string `json:"receiverName"`    // 收件姓名，1-64 个字符。
	ReceiverPhone   string `json:"receiverPhone"`   // 手机号，7-20 个数字，可含前导 +。
	ReceiverAddress string `json:"receiverAddress"` // 收货地址纯文本，1-512 个字符。
}

// HGOrder 是商城实物订单 API 快照；待支付收货信息仅买家可见，付款后的快照仅买家与该店商家可见，不包含钱包流水 ID。
type HGOrder struct {
	ID              uint64 `json:"-"`                   // 内部游标。
	OrderID         string `json:"orderId"`             // 对外交易业务标识。
	MerchantID      string `json:"merchantId"`          // 下单时商家业务标识。
	UserID          string `json:"userId"`              // 买家业务标识，仅授予订单双方。
	ProductID       string `json:"productId"`           // 下单时商品业务标识。
	Title           string `json:"title"`               // 下单时标题快照。
	SKUTitle        string `json:"skuTitle"`            // 下单时规格快照。
	SKUID           string `json:"skuId"`               // 库存使用的 SKU 标识。
	Quantity        uint32 `json:"quantity"`            // 已成交件数，1-10。
	UnitAmount      int64  `json:"unitAmount"`          // 下单时单价，整数 M币。
	Amount          int64  `json:"amount"`              // 支付总额，整数 M币。
	TradeStatus     int8   `json:"tradeStatus"`         // 1待支付、2已支付待发货、3已发货、4确认收货、5超时取消、7未发货已退款。
	ShippingStatus  int8   `json:"shippingStatus"`      // 0待发货、1已发货、2确认收货、3退款关闭。
	ReceiverName    string `json:"receiverName"`        // 收件姓名，仅授权双方可见。
	ReceiverPhone   string `json:"receiverPhone"`       // 收件电话，仅授权双方可见。
	ReceiverAddress string `json:"receiverAddress"`     // 收件地址，仅授权双方可见。
	Carrier         string `json:"carrier"`             // 商家手工填写的承运商名称；空为未发货。
	TrackingNumber  string `json:"trackingNumber"`      // 商家手工填写的运单号，不含第三方轨迹。
	Hash            string `json:"-"`                   // 购买请求 SHA-256 指纹，仅用于服务端幂等比较。
	DebitID         uint64 `json:"-"`                   // 原扣款流水 ID，仅用于未发货原路退款。
	PaymentID       string `json:"-"`                   // 内部成功支付单 ID，仅用于退款事实。
	ExpiresAt       string `json:"expiresAt,omitempty"` // 待支付截止时间，UTC RFC3339；已支付订单不返回。
}

// HGOrderPage 是本人订单或所属店铺订单的单页有界游标协议。
type HGOrderPage struct {
	List       []HGOrder `json:"list"`       // 最多20单，不跨用户或商家。
	NextCursor string    `json:"nextCursor"` // 下一页内部游标；空表示结束。
	HasMore    bool      `json:"hasMore"`    // 超出单页时为 true。
}

// HGShipRequest 是商家人工发货协议；运单号未经快递商核验。
type HGShipRequest struct {
	OrderID        string `json:"orderId"`        // 本店已付款且未发货的订单。
	Carrier        string `json:"carrier"`        // 承运商纯文本名称，1-64字符。
	TrackingNumber string `json:"trackingNumber"` // 运单号，1-128个 ASCII 字母/数字及点、连字符、下划线。
}

// HGRefundRequest 是未发货全额原扣款退款协议，不接受客户端金额或任意扣款流水 ID。
type HGRefundRequest struct {
	OrderID   string `json:"orderId"`   // 本人已付款且未发货的订单。
	RequestID string `json:"requestId"` // 1-64 位 ASCII 幂等键，同请求只可退款一次。
}

// HGOrderStore 定义实物交易的授权读、原子支付和售后操作；具体短事务属于 MySQL 实现。
type HGOrderStore interface {
	Buy(context.Context, string, HGBuyRequest, string) (HGOrder, error)
	PendingOrders(context.Context, string, uint64) ([]HGOrder, error)
	Pay(context.Context, string, string) (HGOrder, error)
	Orders(context.Context, string, bool, uint64) ([]HGOrder, error)
	Order(context.Context, string, string, bool) (HGOrder, error)
	Ship(context.Context, string, HGShipRequest) error
	Receive(context.Context, string, string) error
	Refund(context.Context, string, HGRefundRequest) error
}

func hgBuyHash(req HGBuyRequest) string {
	data, _ := json.Marshal(req) // 固定结构体字段顺序与 JSON tag，包含完整地址快照，仅持久化哈希。
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hgOrderPage(items []HGOrder) HGOrderPage {
	page := HGOrderPage{List: items, HasMore: len(items) > 20}
	if page.HasMore {
		page.List = items[:20]
		page.NextCursor = strconv.FormatUint(page.List[19].ID, 10)
	}
	return page
}

// HGBuy 只接受单 SKU 实物商品，限额由服务端定价和 coin lot 锁上界共同约束。
func (s *HGService) HGBuy(ctx context.Context, user string, req HGBuyRequest) (HGOrder, error) {
	if user == "" || !hgBusinessID(req.ProductID) || !hgBusinessID(req.RequestID) || req.Quantity < 1 || req.Quantity > 10 ||
		!hgText(req.ReceiverName, 64) || !hgText(req.ReceiverAddress, 512) || len(req.ReceiverPhone) < 7 || len(req.ReceiverPhone) > 20 {
		return HGOrder{}, ErrHGInvalid
	}
	for i, c := range req.ReceiverPhone {
		if (c < '0' || c > '9') && !(i == 0 && c == '+') {
			return HGOrder{}, ErrHGInvalid
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.orders.Buy(ctx, user, req, hgBuyHash(req))
}

// HGPendingOrders 仅返回本人待支付或到期取消的预留记录，不暴露商家可见的未付款地址。
func (s *HGService) HGPendingOrders(ctx context.Context, user, cursor string) (HGOrderPage, error) {
	if user == "" {
		return HGOrderPage{}, ErrHGForbidden
	}
	id, err := hgCursor(cursor)
	if err != nil {
		return HGOrderPage{}, err
	}
	if id == 0 {
		id = math.MaxUint64
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	items, err := s.orders.PendingOrders(ctx, user, id)
	if err != nil {
		return HGOrderPage{}, err
	}
	return hgOrderPage(items), nil
}

// HGPay 只允许下单本人按原订单金额在十五分钟内完成 M币扣款；重复请求读取已支付结果。
func (s *HGService) HGPay(ctx context.Context, user, id string) (HGOrder, error) {
	if user == "" || !hgBusinessID(id) {
		return HGOrder{}, ErrHGInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.orders.Pay(ctx, user, id)
}

func (s *HGService) HGOrders(ctx context.Context, user, cursor string, merchant bool) (HGOrderPage, error) {
	if user == "" {
		return HGOrderPage{}, ErrHGForbidden
	}
	id, err := hgCursor(cursor)
	if err != nil {
		return HGOrderPage{}, err
	}
	if id == 0 {
		id = math.MaxUint64
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	items, err := s.orders.Orders(ctx, user, merchant, id)
	if err != nil {
		return HGOrderPage{}, err
	}
	return hgOrderPage(items), nil
}

// HGOrder 按本人或本店身份读取单笔商品订单；买家可读取待支付快照，商家只可读取已付款履约单。
func (s *HGService) HGOrder(ctx context.Context, user, id string, merchant bool) (HGOrder, error) {
	if user == "" || !hgBusinessID(id) {
		return HGOrder{}, ErrHGInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.orders.Order(ctx, user, id, merchant)
}

func (s *HGService) HGShip(ctx context.Context, user string, req HGShipRequest) error {
	if user == "" || !hgBusinessID(req.OrderID) || !hgText(req.Carrier, 64) || len(req.TrackingNumber) == 0 || len(req.TrackingNumber) > 128 {
		return ErrHGInvalid
	}
	for _, c := range req.TrackingNumber {
		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && c != '-' && c != '_' && c != '.' {
			return ErrHGInvalid
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.orders.Ship(ctx, user, req)
}

func (s *HGService) HGReceive(ctx context.Context, user, id string) error {
	if user == "" || !hgBusinessID(id) {
		return ErrHGInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.orders.Receive(ctx, user, id)
}

func (s *HGService) HGRefund(ctx context.Context, user string, req HGRefundRequest) error {
	if user == "" || !hgBusinessID(req.OrderID) || !hgBusinessID(req.RequestID) || strings.TrimSpace(req.RequestID) != req.RequestID {
		return ErrHGInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.orders.Refund(ctx, user, req)
}

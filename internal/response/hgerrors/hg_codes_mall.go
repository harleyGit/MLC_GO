package hgerrors

const (
	HGMallProductNotFound         Code = 1401001
	HGMallProductOffShelf         Code = 1401002
	HGMallSKUNotFound             Code = 1401003
	HGMallStockLockFailed         Code = 1402001
	HGMallStockReservationExpired Code = 1402002
	HGMallPurchaseLimitExceeded   Code = 1402003
	HGMallStockInsufficient       Code = 1402004
	HGMallPurchaseUnavailable     Code = 1402005
	HGMallOrderNotFound           Code = 1403001
	HGMallOrderStatusConflict     Code = 1403002
	HGMallOrderAlreadyPaid        Code = 1403003
	HGMallOrderExpired            Code = 1403004
	HGMallCouponInvalid           Code = 1404001
	HGMallCouponExpired           Code = 1404002
	HGMallAddressRequired         Code = 1404003
	HGMallGroupBuyFull            Code = 1405001
	HGMallGroupBuyExpired         Code = 1405002
)

var hgMall = []Definition{
	{HGMallProductNotFound, "MALL_PRODUCT_NOT_FOUND", "商品不存在", 404, false},
	{HGMallProductOffShelf, "MALL_PRODUCT_OFF_SHELF", "商品已下架", 409, false},
	{HGMallSKUNotFound, "MALL_SKU_NOT_FOUND", "SKU 不存在", 404, false},
	{HGMallStockLockFailed, "MALL_STOCK_LOCK_FAILED", "库存锁定失败", 503, true},
	{HGMallStockReservationExpired, "MALL_STOCK_RESERVATION_EXPIRED", "库存预占已失效", 409, false},
	{HGMallPurchaseLimitExceeded, "MALL_PURCHASE_LIMIT_EXCEEDED", "超过限购数量", 409, false},
	{HGMallStockInsufficient, "MALL_STOCK_INSUFFICIENT", "商品库存不足", 409, false},
	{HGMallPurchaseUnavailable, "MALL_PURCHASE_UNAVAILABLE", "商品当前不可购买", 409, false},
	{HGMallOrderNotFound, "MALL_ORDER_NOT_FOUND", "订单不存在", 404, false},
	{HGMallOrderStatusConflict, "MALL_ORDER_STATUS_CONFLICT", "订单状态不允许操作", 409, false},
	{HGMallOrderAlreadyPaid, "MALL_ORDER_ALREADY_PAID", "订单已支付", 409, false},
	{HGMallOrderExpired, "MALL_ORDER_EXPIRED", "订单已超时", 409, false},
	{HGMallCouponInvalid, "MALL_COUPON_INVALID", "优惠券无效", 400, false},
	{HGMallCouponExpired, "MALL_COUPON_EXPIRED", "优惠券已过期", 409, false},
	{HGMallAddressRequired, "MALL_ADDRESS_REQUIRED", "未选择收货地址", 400, false},
	{HGMallGroupBuyFull, "MALL_GROUP_BUY_FULL", "拼单人数已满", 409, false},
	{HGMallGroupBuyExpired, "MALL_GROUP_BUY_EXPIRED", "拼单已结束", 409, false},
}

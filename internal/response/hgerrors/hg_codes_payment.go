package hgerrors

const (
	HGPaymentOrderNotFound       Code = 1501001
	HGPaymentOrderExpired        Code = 1501002
	HGPaymentChannelUnavailable  Code = 1501003
	HGPaymentIdempotencyConflict Code = 1503001
	HGPaymentRefundNotAllowed    Code = 1503002
	HGPaymentCallbackInvalid     Code = 1503003
)

var hgPayment = []Definition{
	{HGPaymentOrderNotFound, "PAYMENT_ORDER_NOT_FOUND", "支付订单不存在", 404, false},
	{HGPaymentOrderExpired, "PAYMENT_ORDER_EXPIRED", "支付订单已过期", 409, false},
	{HGPaymentChannelUnavailable, "PAYMENT_CHANNEL_UNAVAILABLE", "支付渠道不可用", 503, true},
	{HGPaymentIdempotencyConflict, "PAYMENT_IDEMPOTENCY_CONFLICT", "幂等键对应的请求内容不同", 409, false},
	{HGPaymentRefundNotAllowed, "PAYMENT_REFUND_NOT_ALLOWED", "当前订单不可退款", 409, false},
	{HGPaymentCallbackInvalid, "PAYMENT_CALLBACK_INVALID", "支付回调校验失败", 400, false},
}

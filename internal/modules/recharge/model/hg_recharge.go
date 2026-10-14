package model

import (
	"errors"
	"time"
)

var HGErrInvalid = errors.New("充值记录分页参数无效")
var HGErrInvalidRecord = errors.New("充值记录数据无效")

// HGRecord 是统一充值表的只读数据库模型，不作为 HTTP 响应。
type HGRecord struct {
	HGID          uint64     // 内部主键，游标决胜键。
	HGOrderID     string     // 充值订单业务标识。
	HGTitle       string     // 档位标题快照。
	HGCurrency    string     // 支付币种。
	HGPayAmount   int64      // 支付金额，整数分。
	HGTotalCoin   int64      // 平台币总数，整数。
	HGStatus      int        // 统一表 order_status，0 至 6。
	HGPaymentMode string     // 创建时支付渠道快照。
	HGCreatedAt   time.Time  // UTC 创建时间，毫秒精度。
	HGExpiresAt   time.Time  // UTC 支付截止时间。
	HGPaidAt      *time.Time // UTC 支付时间，未支付为空。
}

// HGCursor 是倒序查询边界，nil 表示首屏。
type HGCursor struct {
	HGCreatedAt time.Time // UTC 毫秒创建时间。
	HGID        uint64    // 同毫秒内的唯一主键。
}

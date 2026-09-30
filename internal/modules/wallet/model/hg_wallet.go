package model

import (
	"errors"
	"time"
)

var (
	HGErrInvalid            = errors.New("钱包参数无效")
	HGErrNotFound           = errors.New("订单不存在或无权访问")
	HGErrSKUUnavailable     = errors.New("充值档位不可用")
	HGErrUnsupported        = errors.New("暂不支持单笔超过1000币的充值")
	HGErrConflict           = errors.New("requestId已绑定其他充值档位")
	HGErrPaymentUnavailable = errors.New("支付渠道未配置，暂不能付款")
)

// HGSKU 是用户充值目录业务模型，不包含运维审计字段。
type HGSKU struct {
	HGID                                                uint64
	HGSKUID, HGTitle, HGCurrency                        string
	HGPayAmount, HGCoinAmount, HGBonusCoin, HGTotalCoin uint64
	HGStartTime                                         time.Time
	HGEndTime                                           *time.Time
}

// HGOrder 保存服务端不可变订单快照；目前没有任何已支付或入账状态。
type HGOrder struct {
	HGOrderID, HGUserID, HGRequestID, HGSKUID         string
	HGDisplayName, HGTitle, HGDescription, HGCurrency string
	HGPayAmount, HGTotalCoin                          uint64
	HGCreatedAt, HGExpiresAt                          time.Time
}

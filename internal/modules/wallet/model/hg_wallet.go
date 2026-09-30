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
	HGErrOrderExpired       = errors.New("充值订单已过期")
)

// HGSKU 是用户充值目录业务模型，不包含运维审计字段。
type HGSKU struct {
	HGID                                                uint64
	HGSKUID, HGTitle, HGCurrency                        string
	HGPayAmount, HGCoinAmount, HGBonusCoin, HGTotalCoin uint64
	HGStartTime                                         time.Time
	HGEndTime                                           *time.Time
}

// HGOrder 保存订单快照及支付状态；资产入账由 coin 权威流水完成。
type HGOrder struct {
	HGOrderID, HGUserID, HGRequestID, HGSKUID         string
	HGDisplayName, HGTitle, HGDescription, HGCurrency string
	HGPaymentMode, HGStatus                           string
	HGPayAmount, HGTotalCoin, HGBalanceAfter          uint64
	HGPaymentTransactionID                            uint64
	HGPaidAt                                          *time.Time
	HGCreatedAt, HGExpiresAt                          time.Time
}

package OpsModelPackage

import "time"

// HGPaymentRechargeSKU 是充值档位的数据库无关业务模型。
type HGPaymentRechargeSKU struct {
	ID         uint64
	SKUID      string
	SKUCode    string
	Title      string
	Currency   string
	PayAmount  uint64
	CoinAmount uint64
	BonusCoin  uint64
	TotalCoin  uint64
	Status     int
	SortOrder  uint64
	StartTime  time.Time
	EndTime    *time.Time
	Version    uint32
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

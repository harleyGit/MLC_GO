package product

import (
	"context"
	"time"
)

// HGProduct 是 product 的数据库模型；商品目录不承担库存、账户或支付职责。
type HGProduct struct {
	ID          uint64    // 数据库内部自增主键。
	ProductID   string    // 商品业务标识。
	ProductType string    // 商品类型，例如 MEMBERSHIP 或 PHYSICAL_PRODUCT。
	Title       string    // 商品名称。
	Currency    string    // 商品标价币种，支持 CNY 与 MLC_COIN。
	Status      int8      // 商品状态：0停用，1启用，2归档。
	CreatedAt   time.Time // 创建时间，UTC 毫秒精度。
	UpdatedAt   time.Time // 更新时间，UTC 毫秒精度。
}

// HGProductSKU 是 product_sku 的数据库模型；amount 为整数最小单位。
type HGProductSKU struct {
	ID        uint64    // 数据库内部自增主键。
	SKUID     string    // SKU 业务标识。
	ProductID string    // 所属商品业务标识。
	SKUCode   string    // SKU 唯一业务编码。
	Title     string    // SKU 展示名称。
	Currency  string    // SKU 定价币种，支持 CNY 与 MLC_COIN。
	Amount    int64     // SKU 单价，整数最小单位。
	Status    int8      // SKU 状态：0停用，1启用，2归档。
	Version   uint32    // SKU 乐观锁版本号，从1开始。
	CreatedAt time.Time // 创建时间，UTC 毫秒精度。
	UpdatedAt time.Time // 更新时间，UTC 毫秒精度。
}

// HGProductRepository 是商品目录持久化契约，当前只定义接口，未接入商品服务。
// FindSKU 未找到须返回可识别错误；下单须校验商品和 SKU 均启用、币种一致并复制快照，不依赖日后目录值。
type HGProductRepository interface {
	FindSKU(ctx context.Context, skuID string) (*HGProductSKU, error)
	CreateProduct(ctx context.Context, product *HGProduct) error
}

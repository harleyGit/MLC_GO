/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-09-30 15:35:06
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-09-30 15:56:34
 * @FilePath: /MLC_GO/internal/modules/ops/dto/hg_payment_recharge_sku_dto.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package OpsDtoPackage

// HGPaymentRechargeSKUCreateRequest 是充值档位创建协议，整数金额单位为分/平台币。
type HGPaymentRechargeSKUCreateRequest struct {
	SKUCode    string  `json:"skuCode"`
	Title      string  `json:"title"`
	PayAmount  *uint64 `json:"payAmount"`
	CoinAmount *uint64 `json:"coinAmount"`
	BonusCoin  *uint64 `json:"bonusCoin"`
	Status     *int    `json:"status"`
	SortOrder  *uint64 `json:"sortOrder"`
	StartTime  string  `json:"startTime"`
	EndTime    *string `json:"endTime"`
}

type HGPaymentRechargeSKUUpdateRequest struct {
	SKUID      string  `json:"skuId"`
	Version    *uint32 `json:"version"`
	SKUCode    string  `json:"skuCode"`
	Title      string  `json:"title"`
	PayAmount  *uint64 `json:"payAmount"`
	CoinAmount *uint64 `json:"coinAmount"`
	BonusCoin  *uint64 `json:"bonusCoin"`
	Status     *int    `json:"status"`
	SortOrder  *uint64 `json:"sortOrder"`
	StartTime  string  `json:"startTime"`
	EndTime    *string `json:"endTime"`
}

type HGPaymentRechargeSKUDeleteRequest struct {
	SKUID   string  `json:"skuId"`
	Version *uint32 `json:"version"`
}

// HGPaymentRechargeSKUItem 充值商品字段，对应表：payment_recharge_sku
type HGPaymentRechargeSKUItem struct {
	SKUID      string `json:"skuId"`
	SKUCode    string `json:"skuCode"`
	Title      string `json:"title"`
	Currency   string `json:"currency"`
	PayAmount  uint64 `json:"payAmount"`
	CoinAmount uint64 `json:"coinAmount"`
	BonusCoin  uint64 `json:"bonusCoin"`
	TotalCoin  uint64 `json:"totalCoin"`
	Status     int    `json:"status"`
	SortOrder  uint64 `json:"sortOrder"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime"`
	Version    uint32 `json:"version"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type HGPaymentRechargeSKUListResponse struct {
	List       []HGPaymentRechargeSKUItem `json:"list"`
	NextCursor string                     `json:"nextCursor"`
	HasMore    bool                       `json:"hasMore"`
	Total      int64                      `json:"total"`
}

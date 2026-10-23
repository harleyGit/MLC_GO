package SQLQueriesPackage

// 商品目录只读生效且审核通过的商品及 SKU；status,id 索引支持倒序游标，单页最多 20+1。
// 与 merchant 按唯一 merchant_id 联查，仅在候选产品页上逐项查询，不作全表扫描。
const HGMallCatalogFirstSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1
WHERE p.status=1 AND p.category=? ORDER BY p.id DESC LIMIT ?`
const HGMallCatalogNextSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1
WHERE p.status=1 AND p.category=? AND p.id<? ORDER BY p.id DESC LIMIT ?`
const HGMallCatalogAllFirstSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1
WHERE p.status=1 AND p.category<>'' ORDER BY p.id DESC LIMIT ?`
const HGMallCatalogAllNextSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1
WHERE p.status=1 AND p.category<>'' AND p.id<? ORDER BY p.id DESC LIMIT ?`

// product_id 与 sku.product_id 分别通过唯一索引和 product_status_id 索引定位；审核后才对外可见。
const HGMallCatalogDetailSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1
WHERE p.product_id=? AND p.status=1 AND p.category<>'' LIMIT 1`
const HGMallMerchantByOwnerSQL = `SELECT merchant_id,shop_name,status FROM mall_merchant WHERE user_id=?`
const HGMallMerchantApplySQL = `INSERT INTO mall_merchant(merchant_id,user_id,shop_name,status) VALUES(?,?,?,0)`

// 唯一 user_id 和 status=2 保证重提申请只作用于自己的已拒绝记录，不允许已通过商家重入。
const HGMallMerchantReapplySQL = `UPDATE mall_merchant SET shop_name=?,status=0,reviewed_by=NULL WHERE user_id=? AND status=2`

// 待审列表按 (status,id) 有界扫描，申请者不能自选审核范围。
const HGMallMerchantPendingSQL = `SELECT id,merchant_id,user_id,shop_name FROM mall_merchant WHERE status=0 AND id<? ORDER BY id DESC LIMIT ?`
const HGMallMerchantApproveSQL = `UPDATE mall_merchant SET status=?,reviewed_by=? WHERE merchant_id=? AND status=0 AND user_id<>?`

// 同一短事务锁定已批准商家并写入一款单 SKU 草稿；未来多 SKU 需要另行评估分页与库存。
const HGMallMerchantApprovedForUpdateSQL = `SELECT merchant_id FROM mall_merchant WHERE user_id=? AND status=1 FOR UPDATE`
const HGMallProductInsertSQL = `INSERT INTO product(product_id,product_type,title,currency,status,merchant_id,category,summary) VALUES(?,?,?,'MLC_COIN',0,?,?,?)`
const HGMallSKUInsertSQL = `INSERT INTO product_sku(sku_id,product_id,sku_code,title,currency,amount,status) VALUES(?,?,?,?,'MLC_COIN',?,1)`
const HGMallMyProductsSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount,p.status
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id JOIN product_sku s ON s.product_id=p.product_id
WHERE m.user_id=? AND p.id<? ORDER BY p.id DESC LIMIT ?`
const HGMallPendingProductsSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount,p.status
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 JOIN product_sku s ON s.product_id=p.product_id
WHERE p.status=0 AND p.merchant_id<>'' AND p.id<? ORDER BY p.id DESC LIMIT ?`
const HGMallProductReviewSQL = `UPDATE product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 SET p.status=? WHERE p.product_id=? AND p.status=0 AND p.merchant_id<>''`

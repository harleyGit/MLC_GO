package SQLQueriesPackage

// 商品目录只读生效且审核通过的商品及 SKU；(status,category,id) 与 (review_status,id) 支持倒序游标，单页最多 20+1。
// 对每个商品按 (product_id,status,id) 只选最早启用的 SKU，避免多规格重复占用商品游标；生产执行计划仍须以真实索引与数据验证。
const HGMallCatalogFirstSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount,COALESCE(i.available,0),p.description,p.image_urls
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1 AND s.id=(SELECT s2.id FROM product_sku s2 WHERE s2.product_id=p.product_id AND s2.status=1 ORDER BY s2.id LIMIT 1)
LEFT JOIN mall_inventory i ON i.sku_id=s.sku_id
WHERE p.status=1 AND p.review_status=1 AND p.category=? ORDER BY p.id DESC LIMIT ?`
const HGMallCatalogNextSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount,COALESCE(i.available,0),p.description,p.image_urls
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1 AND s.id=(SELECT s2.id FROM product_sku s2 WHERE s2.product_id=p.product_id AND s2.status=1 ORDER BY s2.id LIMIT 1)
LEFT JOIN mall_inventory i ON i.sku_id=s.sku_id
WHERE p.status=1 AND p.review_status=1 AND p.category=? AND p.id<? ORDER BY p.id DESC LIMIT ?`
const HGMallCatalogAllFirstSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount,COALESCE(i.available,0),p.description,p.image_urls
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1 AND s.id=(SELECT s2.id FROM product_sku s2 WHERE s2.product_id=p.product_id AND s2.status=1 ORDER BY s2.id LIMIT 1)
LEFT JOIN mall_inventory i ON i.sku_id=s.sku_id
WHERE p.status=1 AND p.review_status=1 AND p.category<>'' ORDER BY p.id DESC LIMIT ?`
const HGMallCatalogAllNextSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount,COALESCE(i.available,0),p.description,p.image_urls
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1 AND s.id=(SELECT s2.id FROM product_sku s2 WHERE s2.product_id=p.product_id AND s2.status=1 ORDER BY s2.id LIMIT 1)
LEFT JOIN mall_inventory i ON i.sku_id=s.sku_id
WHERE p.status=1 AND p.review_status=1 AND p.category<>'' AND p.id<? ORDER BY p.id DESC LIMIT ?`

// product_id 与 sku.product_id 分别通过唯一索引和 product_status_id 索引定位；审核后才对外可见。
const HGMallCatalogDetailSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount,COALESCE(i.available,0),p.description,p.image_urls
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1 AND s.id=(SELECT s2.id FROM product_sku s2 WHERE s2.product_id=p.product_id AND s2.status=1 ORDER BY s2.id LIMIT 1)
LEFT JOIN mall_inventory i ON i.sku_id=s.sku_id
WHERE p.product_id=? AND p.status=1 AND p.review_status=1 AND p.category<>'' LIMIT 1`
const HGMallMerchantByOwnerSQL = `SELECT merchant_id,shop_name,status FROM mall_merchant WHERE user_id=?`
const HGMallMerchantApplySQL = `INSERT INTO mall_merchant(merchant_id,user_id,shop_name,status) VALUES(?,?,?,0)`

// 唯一 user_id 和 status=2 保证重提申请只作用于自己的已拒绝记录，不允许已通过商家重入。
const HGMallMerchantReapplySQL = `UPDATE mall_merchant SET shop_name=?,status=0,reviewed_by=NULL WHERE user_id=? AND status=2`

// 待审列表按 (status,id) 有界扫描，申请者不能自选审核范围。
const HGMallMerchantPendingSQL = `SELECT id,merchant_id,user_id,shop_name FROM mall_merchant WHERE status=0 AND id<? ORDER BY id DESC LIMIT ?`
const HGMallMerchantApproveSQL = `UPDATE mall_merchant SET status=?,reviewed_by=? WHERE merchant_id=? AND status=0 AND user_id<>?`

// 同一短事务锁定已批准商家并写入一款单 SKU 草稿；未来多 SKU 需要另行评估分页与库存。
const HGMallMerchantApprovedForUpdateSQL = `SELECT merchant_id FROM mall_merchant WHERE user_id=? AND status=1 FOR UPDATE`
const HGMallProductInsertSQL = `INSERT INTO product(product_id,product_type,title,currency,status,review_status,merchant_id,category,summary,description,image_urls) VALUES(?,?,?,'MLC_COIN',0,0,?,?,?,?,?)`

// 图片唯一 URL 点查并限制未绑定状态，需在商品短事务中完成，不在事务内访问对象存储。
const HGMallImageBindSQL = `UPDATE mall_product_image SET product_id=?,status=2 WHERE image_url_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=1`

// 每个用户未绑定图片最多 20 张，避免反复上传不发布导致无界对象增长；依赖 (user_id,status,id) 索引。
const HGMallImageReserveSQL = `INSERT INTO mall_product_image(image_url,storage_key,user_id,status) SELECT ?,?,?,0 WHERE (SELECT COUNT(*) FROM mall_product_image WHERE user_id=? AND status IN (0,1))<20`
const HGMallImageReadySQL = `UPDATE mall_product_image SET status=1 WHERE image_url_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=0`
const HGMallImageRemoveSQL = `DELETE FROM mall_product_image WHERE image_url_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=0`
const HGMallImageClaimDeleteSQL = `UPDATE mall_product_image SET status=3 WHERE image_url_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=1`
const HGMallImageKeySQL = `SELECT storage_key FROM mall_product_image WHERE image_url_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=3`
const HGMallImageFinishDeleteSQL = `DELETE FROM mall_product_image WHERE image_url_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=3`
const HGMallImageUndoDeleteSQL = `UPDATE mall_product_image SET status=1 WHERE image_url_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=3`

// 本人未发布图片最多20张；按 (user_id,status,id) 覆盖索引稳定倒序读取，无跨用户扫描。
const HGMallImageUnusedListSQL = `SELECT image_url FROM mall_product_image WHERE user_id=? AND status IN (1,3) ORDER BY id DESC LIMIT 20`
const HGMallSKUInsertSQL = `INSERT INTO product_sku(sku_id,product_id,sku_code,title,currency,amount,status) VALUES(?,?,?,?,'MLC_COIN',?,1)`
const HGMallInventoryInsertSQL = `INSERT INTO mall_inventory(sku_id,available) VALUES(?,?)`
const HGMallMyProductsSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount,COALESCE(i.available,0),p.review_status,p.description,p.image_urls
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id JOIN product_sku s ON s.product_id=p.product_id AND s.id=(SELECT s2.id FROM product_sku s2 WHERE s2.product_id=p.product_id ORDER BY s2.id LIMIT 1) LEFT JOIN mall_inventory i ON i.sku_id=s.sku_id
WHERE m.user_id=? AND p.id<? ORDER BY p.id DESC LIMIT ?`
const HGMallPendingProductsSQL = `SELECT p.id,p.product_id,p.product_type,p.title,p.category,p.summary,p.merchant_id,m.shop_name,s.sku_id,s.title,s.amount,COALESCE(i.available,0),p.review_status,p.description,p.image_urls
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 JOIN product_sku s ON s.product_id=p.product_id AND s.id=(SELECT s2.id FROM product_sku s2 WHERE s2.product_id=p.product_id ORDER BY s2.id LIMIT 1) LEFT JOIN mall_inventory i ON i.sku_id=s.sku_id
WHERE p.review_status=0 AND p.status=0 AND p.id<? ORDER BY p.id DESC LIMIT ?`

// 仅审核待审商品，审核者不能批准或拒绝本人店铺的商品；按 product_id 唯一点查。
const HGMallProductReviewSQL = `UPDATE product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 SET p.status=?,p.review_status=? WHERE p.product_id=? AND p.status=0 AND p.review_status=0 AND m.user_id<>?`

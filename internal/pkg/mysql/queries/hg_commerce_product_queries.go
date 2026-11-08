package SQLQueriesPackage

// 生产 CNY 商品 SQL 仅访问 commerce_*；所有 ID 点查走唯一键，列表走 (merchant_id,id)/(category_id,sale_status,id)/(audit_status,id)，单页最多 21 行。
const (
	HGCommerceMerchantSQL           = `SELECT merchant_id FROM mall_merchant WHERE user_id=? AND status=1`
	HGCommerceMerchantLockSQL       = `SELECT merchant_id FROM mall_merchant WHERE user_id=? AND status=1 FOR UPDATE`
	HGCommerceCategorySQL           = `SELECT id,name,parent_id,depth,image_url FROM commerce_category WHERE status=1 AND regulated=0 AND NOT EXISTS (SELECT 1 FROM commerce_category child WHERE child.parent_id=commerce_category.id AND child.status=1) ORDER BY id LIMIT 200`
	HGCommerceCategoryParentSQL     = `SELECT depth,regulated FROM commerce_category WHERE id=? AND status=1 FOR UPDATE`
	HGCommerceCategoryInsertSQL     = `INSERT INTO commerce_category(parent_id,depth,name,regulated,image_url) VALUES(?,?,?,?,?)`
	HGCommerceCategoryLeafSQL       = `SELECT id FROM commerce_category WHERE id=? AND status=1 AND regulated=0 AND NOT EXISTS (SELECT 1 FROM commerce_category child WHERE child.parent_id=commerce_category.id AND child.status=1) LIMIT 1`
	HGCommerceCreateReplaySQL       = `SELECT request_hash,product_id FROM commerce_publish_request WHERE merchant_id=? AND request_id=?`
	HGCommerceCreateProductSQL      = `INSERT INTO commerce_product(product_id,merchant_id,category_id) VALUES(?,?,?)`
	HGCommerceCreateVersionSQL      = `INSERT INTO commerce_product_version(product_id,content_version,title,subtitle,description,images_json) VALUES(?,?,?,?,?,?)`
	HGCommerceCreateSKUSQL          = `INSERT INTO commerce_sku(sku_id,product_id,content_version,sku_code,specs_json,spec_hash,price_cent,initial_stock) VALUES(?,?,?,?,?,?,?,?)`
	HGCommerceBindMediaSQL          = `UPDATE commerce_media SET product_id=?,status=2 WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=1`
	HGCommerceMediaReuseSQL         = `SELECT image_url FROM commerce_media WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND product_id=? AND status=2`
	HGCommerceCreateKeySQL          = `INSERT INTO commerce_publish_request(merchant_id,request_id,request_hash,product_id) VALUES(?,?,?,?)`
	HGCommerceOwnerLockSQL          = `SELECT p.merchant_id,p.category_id,p.audit_status,p.sale_status,p.version,p.draft_version,p.published_version FROM commerce_product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 WHERE p.product_id=? AND m.user_id=? FOR UPDATE`
	HGCommerceOwnerSQL              = `SELECT p.merchant_id,p.category_id,p.audit_status,p.sale_status,p.version,p.draft_version,p.published_version FROM commerce_product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 WHERE p.product_id=? AND m.user_id=?`
	HGCommerceProductStateSQL       = `UPDATE commerce_product SET audit_status=?,sale_status=?,version=version+1,draft_version=?,published_version=? WHERE product_id=? AND version=?`
	HGCommerceProductCategorySQL    = `UPDATE commerce_product SET category_id=?,version=version+1,draft_version=?,audit_status='DRAFT' WHERE product_id=? AND version=?`
	HGCommerceDraftStatusSQL        = `UPDATE commerce_product SET audit_status='PENDING',version=version+1 WHERE product_id=? AND audit_status='DRAFT' AND version=?`
	HGCommerceReviewLockSQL         = `SELECT p.audit_status,p.sale_status,p.version,p.draft_version,p.published_version,m.user_id,p.category_id FROM commerce_product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 WHERE p.product_id=? FOR UPDATE`
	HGCommerceReviewDetailSQL       = `SELECT p.id,p.product_id,p.merchant_id,p.category_id,p.audit_status,p.sale_status,p.version,p.draft_version,p.published_version FROM commerce_product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 WHERE p.product_id=? AND p.audit_status='PENDING'`
	HGCommerceReviewVersionSQL      = `UPDATE commerce_product_version SET reviewed_by=?,reject_reason=? WHERE product_id=? AND content_version=? AND reviewed_by IS NULL`
	HGCommerceMyPageSQL             = `SELECT p.id,p.product_id,p.merchant_id,p.category_id,p.audit_status,p.sale_status,p.version,p.draft_version,p.published_version,v.title,v.images_json,(SELECT MIN(s.price_cent) FROM commerce_sku s WHERE s.product_id=p.product_id AND s.content_version=p.draft_version) FROM commerce_product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 JOIN commerce_product_version v ON v.product_id=p.product_id AND v.content_version=p.draft_version WHERE m.user_id=? AND p.id<? ORDER BY p.id DESC LIMIT 21`
	HGCommerceReviewPageSQL         = `SELECT p.id,p.product_id,p.merchant_id,p.category_id,p.audit_status,p.sale_status,p.version,p.draft_version,p.published_version,v.title,v.images_json,(SELECT MIN(s.price_cent) FROM commerce_sku s WHERE s.product_id=p.product_id AND s.content_version=p.draft_version) FROM commerce_product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 JOIN commerce_product_version v ON v.product_id=p.product_id AND v.content_version=p.draft_version WHERE p.audit_status='PENDING' AND p.id<? ORDER BY p.id DESC LIMIT 21`
	HGCommercePublicPageSQL         = `SELECT p.id,p.product_id,p.merchant_id,p.category_id,p.audit_status,p.sale_status,p.version,p.draft_version,p.published_version,v.title,v.images_json,(SELECT MIN(s.price_cent) FROM commerce_sku s WHERE s.product_id=p.product_id AND s.content_version=p.published_version) FROM commerce_product p JOIN commerce_product_version v ON v.product_id=p.product_id AND v.content_version=p.published_version JOIN commerce_category c ON c.id=p.category_id AND c.status=1 AND c.regulated=0 JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 WHERE p.sale_status='ON_SHELF' AND p.published_version>0 AND p.id<? ORDER BY p.id DESC LIMIT 21`
	HGCommerceVersionSQL            = `SELECT title,subtitle,description,images_json,reject_reason FROM commerce_product_version WHERE product_id=? AND content_version=?`
	HGCommerceSKUsSQL               = `SELECT sku_id,sku_code,specs_json,price_cent,initial_stock FROM commerce_sku WHERE product_id=? AND content_version=? ORDER BY id LIMIT 201`
	HGCommercePublicOneSQL          = `SELECT p.id,p.product_id,p.merchant_id,p.category_id,p.audit_status,p.sale_status,p.version,p.draft_version,p.published_version FROM commerce_product p JOIN commerce_category c ON c.id=p.category_id AND c.status=1 AND c.regulated=0 JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 WHERE p.product_id=? AND p.sale_status='ON_SHELF' AND p.published_version>0`
	HGCommerceMediaCountSQL         = `SELECT COUNT(*) FROM commerce_media WHERE user_id=? AND status IN (0,1)`
	HGCommerceMediaReserveSQL       = `INSERT INTO commerce_media(user_id,image_url,storage_key,status) VALUES(?,?,?,0)`
	HGCommerceMediaReadySQL         = `UPDATE commerce_media SET status=1 WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=0`
	HGCommerceMediaRemovePendingSQL = `DELETE FROM commerce_media WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=0`
	HGCommerceMediaClaimDeleteSQL   = `UPDATE commerce_media SET status=3 WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=1`
	HGCommerceMediaKeySQL           = `SELECT storage_key FROM commerce_media WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=3`
	HGCommerceMediaFinishDeleteSQL  = `DELETE FROM commerce_media WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=3`
	HGCommerceMediaUndoDeleteSQL    = `UPDATE commerce_media SET status=1 WHERE image_hash=UNHEX(SHA2(?,256)) AND image_url=? AND user_id=? AND status=3`
	HGCommerceMediaUnusedSQL        = `SELECT image_url FROM commerce_media WHERE user_id=? AND status IN (1,3) ORDER BY id DESC LIMIT 20`
)

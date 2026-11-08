package SQLQueriesPackage

// 仅由显式启用的隔离 debug 集成测试调用；所有删除都按本次随机业务 ID 或测试买家精确约束。
const (
	HGCommerceFixtureUserSQL              = `INSERT INTO users(user_id,user_name,password_hash,salt) VALUES(?,?,?,?)`
	HGCommerceFixtureMerchantSQL          = `INSERT INTO mall_merchant(merchant_id,user_id,shop_name,status) VALUES(?,?,?,1)`
	HGCommerceFixtureCategorySQL          = `INSERT INTO commerce_category(depth,name,status,regulated) VALUES(1,?,1,0)`
	HGCommerceFixtureProductSQL           = `INSERT INTO commerce_product(product_id,merchant_id,category_id,audit_status,sale_status,published_version) VALUES(?,?,?,'APPROVED','ON_SHELF',1)`
	HGCommerceFixtureVersionSQL           = `INSERT INTO commerce_product_version(product_id,content_version,title,images_json) VALUES(?,1,?,JSON_ARRAY())`
	HGCommerceFixtureSKUSQL               = `INSERT INTO commerce_sku(sku_id,product_id,content_version,sku_code,specs_json,spec_hash,price_cent,initial_stock) VALUES(?,?,1,'BLACK',JSON_OBJECT(),UNHEX(SHA2('{}',256)),100,10)`
	HGCommerceFixtureStockSQL             = `INSERT INTO commerce_inventory(product_id,sku_code,available) VALUES(?,'BLACK',10)`
	HGCommerceFixtureAddressSQL           = `INSERT INTO user_addresses(address_id,user_id,receiver_name,receiver_phone,region,province_code,city_code,detail_address) VALUES(?,?,?,?,?,?,?,?)`
	HGCommerceFixtureMediaSQL             = `INSERT INTO commerce_media(user_id,image_url,storage_key,status) VALUES(?,?,?,1)`
	HGCommerceFixtureFindDraftSQL         = `SELECT product_id FROM commerce_publish_request WHERE merchant_id=? AND request_id=?`
	HGCommerceFixtureFindSKUSQL           = `SELECT sku_id FROM commerce_sku WHERE product_id=? AND content_version=1 AND sku_code='BLACK'`
	HGCommerceFixtureOrderIDsSQL          = `SELECT trade_order_id FROM commerce_order WHERE user_id=? ORDER BY id DESC LIMIT 20`
	HGCommerceFixtureOrderByRequestSQL    = `SELECT trade_order_id FROM commerce_order WHERE user_id=? AND request_id=?`
	HGCommerceFixtureStockReadSQL         = `SELECT available,reserved,sold FROM commerce_inventory WHERE product_id=? AND sku_code='BLACK'`
	HGCommerceFixtureStockAuditCountSQL   = `SELECT COUNT(*) FROM commerce_inventory_adjustment WHERE product_id=? AND actor_user_id=?`
	HGCommerceFixtureExpireOrderSQL       = `UPDATE commerce_order SET expire_at=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND WHERE trade_order_id=? AND user_id=? AND status=1`
	HGCommerceFixtureRemoveReservationSQL = `DELETE FROM commerce_inventory_reservation WHERE trade_order_id=?`
	HGCommerceFixtureRemoveSimEventSQL    = `DELETE FROM commerce_simulated_payment_event WHERE trade_order_id=?`
	HGCommerceFixtureRemoveSimRefundSQL   = `DELETE FROM refund_order WHERE trade_order_id=? AND user_id=? AND request_id=?`
	HGCommerceFixtureRemoveSimPaymentSQL  = `DELETE FROM payment_order WHERE trade_order_id=? AND user_id=? AND channel='SIMULATED'`
	HGCommerceFixtureRemoveOrderSQL       = `DELETE FROM commerce_order WHERE trade_order_id=? AND user_id=?`
	HGCommerceFixtureRemoveItemSQL        = `DELETE FROM trade_order_item WHERE trade_order_id=? AND user_id=?`
	HGCommerceFixtureRemoveTradeSQL       = `DELETE FROM trade_order WHERE trade_order_id=? AND user_id=?`
	HGCommerceFixtureRemoveAddressSQL     = `DELETE FROM user_addresses WHERE address_id=? AND user_id=?`
	HGCommerceFixtureRemoveStockSQL       = `DELETE FROM commerce_inventory WHERE product_id=? AND sku_code='BLACK'`
	HGCommerceFixtureRemoveStockAuditSQL  = `DELETE FROM commerce_inventory_adjustment WHERE product_id=? AND actor_user_id=?`
	HGCommerceFixtureRemoveSKUSQL         = `DELETE FROM commerce_sku WHERE product_id=?`
	HGCommerceFixtureRemoveVersionSQL     = `DELETE FROM commerce_product_version WHERE product_id=?`
	HGCommerceFixtureRemoveProductSQL     = `DELETE FROM commerce_product WHERE product_id=?`
	HGCommerceFixtureRemovePublishKeySQL  = `DELETE FROM commerce_publish_request WHERE merchant_id=? AND request_id=?`
	HGCommerceFixtureRemoveMediaSQL       = `DELETE FROM commerce_media WHERE user_id=? AND image_url=?`
	HGCommerceFixtureRemoveCategorySQL    = `DELETE FROM commerce_category WHERE id=?`
	HGCommerceFixtureRemoveMerchantSQL    = `DELETE FROM mall_merchant WHERE merchant_id=?`
	HGCommerceFixtureRemoveUserSQL        = `DELETE FROM users WHERE user_id=?`
)

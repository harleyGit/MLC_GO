package SQLQueriesPackage

// 生产 CNY 库存和订单始终点查商家/商品/SKU；仅过期扫描使用 (status,expire_at,id) 前缀且单批最多20单。
// 订单写入锁序为商品行、库存行（条件更新），关闭与未来支付回调锁序为订单行、预占行、库存行。
const (
	// 审核通过只登记可经营 SKU 的零可售库存；同编码跨版本复用，不信任草稿 initial_stock。
	HGCommerceInventoryEnsureSQL = `INSERT INTO commerce_inventory(product_id,sku_code) SELECT product_id,sku_code FROM commerce_sku WHERE product_id=? AND content_version=? ON DUPLICATE KEY UPDATE product_id=VALUES(product_id)`
	// 报价只读取发布版本及权威库存；唯一 sku_id / product_id、商家唯一键和类目主键约束连接成本。
	HGCommerceOrderQuoteSQL = `SELECT p.merchant_id,v.title,s.sku_code,s.specs_json,s.price_cent,COALESCE(i.available,0) FROM commerce_product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1 JOIN commerce_category c ON c.id=p.category_id AND c.status=1 AND c.regulated=0 JOIN commerce_product_version v ON v.product_id=p.product_id AND v.content_version=p.published_version JOIN commerce_sku s ON s.product_id=p.product_id AND s.content_version=p.published_version AND s.sku_id=? LEFT JOIN commerce_inventory i ON i.product_id=p.product_id AND i.sku_code=s.sku_code WHERE p.product_id=? AND p.sale_status='ON_SHELF' AND p.published_version>0`
	// 单订单只读本人已登记地址，快照在相同事务内写入；地址若被删除则创建失败。
	HGCommerceOrderAddressSQL       = `SELECT receiver_name,receiver_phone,region,detail_address FROM user_addresses WHERE address_id=? AND user_id=?`
	HGCommerceOrderBuyerMerchantSQL = `SELECT 1 FROM mall_merchant WHERE merchant_id=? AND user_id=? LIMIT 1`
	HGCommerceOrderReplaySQL        = `SELECT trade_order_id,request_hash FROM commerce_order WHERE user_id=? AND request_id=?`
	HGCommerceOrderProductLockSQL   = `SELECT id FROM commerce_product WHERE product_id=? AND sale_status='ON_SHELF' FOR UPDATE`
	// 唯一 (product_id,sku_code) 条件更新杜绝超卖；库存行锁不会跨 SKU 扩散，售罄返回冲突。
	HGCommerceStockReserveSQL      = `UPDATE commerce_inventory SET available=available-?,reserved=reserved+?,version=version+1 WHERE product_id=? AND sku_code=? AND available>=?`
	HGCommerceStockReleaseSQL      = `UPDATE commerce_inventory SET available=available+?,reserved=reserved-?,version=version+1 WHERE product_id=? AND sku_code=? AND reserved>=?`
	HGCommerceStockAdjustSQL       = `UPDATE commerce_inventory SET available=available+?,version=version+1 WHERE product_id=? AND sku_code=? AND version=? AND available+?>=0 AND available+?<=100000`
	HGCommerceStockOwnedLockSQL    = `SELECT i.version FROM commerce_inventory i JOIN commerce_product p ON p.product_id=i.product_id JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.user_id=? AND m.status=1 JOIN commerce_sku s ON s.product_id=p.product_id AND s.content_version=p.published_version AND s.sku_code=i.sku_code WHERE i.product_id=? AND i.sku_code=? AND p.sale_status='ON_SHELF' FOR UPDATE`
	HGCommerceTradeCreateSQL       = `INSERT INTO trade_order(trade_order_id,user_id,request_id,request_hash,order_type,title,currency,total_amount,discount_amount,payable_amount,trade_status,source,expire_at) VALUES(?,?,?,?,'PRODUCT',?,'CNY',?,0,?,1,'WEB',?)`
	HGCommerceTradeItemSQL         = `INSERT INTO trade_order_item(order_item_id,trade_order_id,line_no,user_id,product_id,sku_id,product_title,sku_title,quantity,unit_amount,total_amount) VALUES(?,?,1,?,?,?,?,?,?,?,?)`
	HGCommerceOrderCreateSQL       = `INSERT INTO commerce_order(trade_order_id,user_id,merchant_id,product_id,sku_code,sku_id,title,specs_json,quantity,unit_price_cent,total_cent,request_id,request_hash,address_id,receiver_name,receiver_phone,receiver_address,expire_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	HGCommerceReservationCreateSQL = `INSERT INTO commerce_inventory_reservation(reservation_id,trade_order_id,product_id,sku_code,quantity) VALUES(?,?,?,?,?)`
	HGCommerceOrderOwnerSQL        = `SELECT id,trade_order_id,user_id,merchant_id,product_id,sku_id,sku_code,title,specs_json,quantity,unit_price_cent,total_cent,status,expire_at,address_id,receiver_name,receiver_phone,receiver_address,carrier,tracking_number FROM commerce_order WHERE trade_order_id=? AND user_id=?`
	HGCommerceOrderBuyerPageSQL    = `SELECT id,trade_order_id,user_id,merchant_id,product_id,sku_id,sku_code,title,specs_json,quantity,unit_price_cent,total_cent,status,expire_at FROM commerce_order WHERE user_id=? AND id<? ORDER BY id DESC LIMIT 21`
	// 商家仅能读取支付已确认的订单及收货快照；待支付订单、地址不进入商家页面。
	// 本店跨多个已支付状态的游标查询依赖 000065 的 (merchant_id,id)；避免跨状态 filesort 扫描。
	HGCommerceOrderSellerPageSQL    = `SELECT o.id,o.trade_order_id,o.user_id,o.merchant_id,o.product_id,o.sku_id,o.sku_code,o.title,o.specs_json,o.quantity,o.unit_price_cent,o.total_cent,o.status,o.expire_at FROM commerce_order o JOIN mall_merchant m ON m.merchant_id=o.merchant_id AND m.user_id=? AND m.status=1 WHERE o.status IN(2,3,4) AND o.id<? ORDER BY o.id DESC LIMIT 21`
	HGCommerceOrderSellerOneSQL     = `SELECT o.id,o.trade_order_id,o.user_id,o.merchant_id,o.product_id,o.sku_id,o.sku_code,o.title,o.specs_json,o.quantity,o.unit_price_cent,o.total_cent,o.status,o.expire_at,o.address_id,o.receiver_name,o.receiver_phone,o.receiver_address,o.carrier,o.tracking_number FROM commerce_order o JOIN mall_merchant m ON m.merchant_id=o.merchant_id AND m.user_id=? AND m.status=1 WHERE o.trade_order_id=? AND o.status IN(2,3,4)`
	HGCommerceOrderCloseLockSQL     = `SELECT o.user_id,o.product_id,o.sku_code,o.quantity,o.status,o.expire_at FROM commerce_order o WHERE o.trade_order_id=? FOR UPDATE`
	HGCommerceReservationReleaseSQL = `UPDATE commerce_inventory_reservation SET status=2 WHERE trade_order_id=? AND status=1`
	HGCommerceOrderCloseSQL         = `UPDATE commerce_order SET status=5 WHERE trade_order_id=? AND status=1`
	HGCommerceTradeCloseSQL         = `UPDATE trade_order SET trade_status=5,cancelled_at=UTC_TIMESTAMP(3) WHERE trade_order_id=? AND currency='CNY' AND trade_status=1`
	// 扫描只拿游标、逐单在短事务内二次判断状态；无外部网络 I/O，不按 offset 扫描大表。
	HGCommerceOrderExpiredPageSQL = `SELECT trade_order_id FROM commerce_order WHERE status=1 AND expire_at<=UTC_TIMESTAMP(3) ORDER BY expire_at,id LIMIT 20`
	// 后续渠道必须在同一订单锁下落地已验签支付事实；当前仅允许读取已有真实 CNY 成功支付事实。
	HGCommerceShipLockSQL   = `SELECT o.status FROM commerce_order o JOIN mall_merchant m ON m.merchant_id=o.merchant_id AND m.user_id=? AND m.status=1 JOIN trade_order t ON t.trade_order_id=o.trade_order_id AND t.currency='CNY' AND t.trade_status=2 JOIN payment_order pay ON pay.trade_order_id=o.trade_order_id AND pay.currency='CNY' AND pay.payment_status=2 AND pay.channel IN ('ALIPAY','WECHAT') AND pay.external_payment_id IS NOT NULL WHERE o.trade_order_id=? FOR UPDATE`
	HGCommerceShipUpdateSQL = `UPDATE commerce_order SET status=3,carrier=?,tracking_number=?,shipped_at=UTC_TIMESTAMP(3) WHERE trade_order_id=? AND status=2`
	HGCommerceShipTradeSQL  = `UPDATE trade_order SET trade_status=3 WHERE trade_order_id=? AND currency='CNY' AND trade_status=2`
)

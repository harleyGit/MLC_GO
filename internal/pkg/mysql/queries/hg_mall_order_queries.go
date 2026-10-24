package SQLQueriesPackage

// 创建订单前以商品唯一 product_id 定位已审核、启用的单规格 SKU 与商家，锁库存行后再锁钱包。
// 每单一款商品、1-10 件且金额至多100000 M币；既有旧商品无库存记录不可购买。
const HGMallBuyProductSQL = `SELECT p.product_id,p.title,p.merchant_id,m.user_id,s.sku_id,s.title,s.amount,i.available
FROM product p JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
JOIN product_sku s ON s.product_id=p.product_id AND s.status=1 AND s.id=(SELECT s2.id FROM product_sku s2 WHERE s2.product_id=p.product_id AND s2.status=1 ORDER BY s2.id LIMIT 1)
JOIN mall_inventory i ON i.sku_id=s.sku_id
WHERE p.product_id=? AND p.product_type='PHYSICAL_PRODUCT' AND p.status=1 AND p.review_status=1 AND s.currency='MLC_COIN' LIMIT 1 FOR UPDATE`

// 库存基于 sku_id 主键有界条件更新，并发下不足时受影响行数为零；事务失败全量回滚。
const HGMallTakeStockSQL = `UPDATE mall_inventory SET available=available-?,sold=sold+? WHERE sku_id=? AND available>=? AND sold<=4294967295-?`
const HGMallReturnStockSQL = `UPDATE mall_inventory SET available=available+?,sold=sold-? WHERE sku_id=? AND sold>=? AND available<=4294967295-?`
const HGMallTradeInsertSQL = `INSERT INTO trade_order(trade_order_id,user_id,request_id,request_hash,order_type,title,currency,total_amount,discount_amount,payable_amount,trade_status,source,paid_at) VALUES(?,?,?,?,'PRODUCT',?,'MLC_COIN',?,0,?,2,'WEB',UTC_TIMESTAMP(3))`
const HGMallItemInsertSQL = `INSERT INTO trade_order_item(order_item_id,trade_order_id,line_no,user_id,product_id,sku_id,product_title,sku_title,quantity,unit_amount,total_amount) VALUES(?,?,1,?,?,?,?,?,?,?,?)`
const HGMallPaymentInsertSQL = `INSERT INTO payment_order(payment_id,trade_order_id,user_id,request_id,request_hash,payment_type,currency,amount,payment_status,expire_at,paid_at) VALUES(?,?,?,?,?,'COIN','MLC_COIN',?,2,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`
const HGMallFulfillmentInsertSQL = `INSERT INTO mall_fulfillment(trade_order_id,merchant_id,user_id,sku_id,payment_id,quantity,debit_transaction_id,receiver_name,receiver_phone,receiver_address) VALUES(?,?,?,?,?,?,?,?,?,?)`

// 购买热路径先按 trade_order 的 uk_trade_order_request_user 检查幂等键；首次购买不做订单明细与地址联查。
// 请求重放再按订单业务 ID 和买家身份读取完整快照；写入结果未知时继续以同一请求键查询，不能再次扣款。
const HGMallBuyReplayKeySQL = `SELECT trade_order_id,request_hash FROM trade_order WHERE user_id=? AND request_id=? LIMIT 1`

// 订单主键、本人唯一请求键、(user_id,id) 和 (merchant_id,id) 游标保证可重复查询，地址仅通过买家或店主作用域返回。
const HGMallOrderSelectFieldsSQL = `SELECT f.id,f.trade_order_id,f.merchant_id,f.user_id,ti.product_id,ti.product_title,ti.sku_title,f.sku_id,f.quantity,ti.unit_amount,t.payable_amount,t.trade_status,f.shipping_status,f.receiver_name,f.receiver_phone,f.receiver_address,f.carrier,f.tracking_number,t.request_hash,f.debit_transaction_id,f.payment_id`
const HGMallOrderByOwnerSQL = HGMallOrderSelectFieldsSQL + ` FROM mall_fulfillment f JOIN trade_order t ON t.trade_order_id=f.trade_order_id JOIN trade_order_item ti ON ti.trade_order_id=t.trade_order_id AND ti.line_no=1 WHERE f.user_id=? AND f.trade_order_id=? LIMIT 1`
const HGMallOrderByMerchantSQL = HGMallOrderSelectFieldsSQL + ` FROM mall_fulfillment f JOIN mall_merchant m ON m.merchant_id=f.merchant_id JOIN trade_order t ON t.trade_order_id=f.trade_order_id JOIN trade_order_item ti ON ti.trade_order_id=t.trade_order_id AND ti.line_no=1 WHERE m.user_id=? AND m.status=1 AND f.trade_order_id=? LIMIT 1`
const HGMallBuyerOrdersSQL = HGMallOrderSelectFieldsSQL + ` FROM mall_fulfillment f JOIN trade_order t ON t.trade_order_id=f.trade_order_id JOIN trade_order_item ti ON ti.trade_order_id=t.trade_order_id AND ti.line_no=1 WHERE f.user_id=? AND f.id<? ORDER BY f.id DESC LIMIT ?`
const HGMallSellerOrdersSQL = HGMallOrderSelectFieldsSQL + ` FROM mall_fulfillment f JOIN mall_merchant m ON m.merchant_id=f.merchant_id AND m.status=1 JOIN trade_order t ON t.trade_order_id=f.trade_order_id JOIN trade_order_item ti ON ti.trade_order_id=t.trade_order_id AND ti.line_no=1 WHERE m.user_id=? AND f.id<? ORDER BY f.id DESC LIMIT ?`

// 变更先按订单唯一键 SELECT ... FOR UPDATE 锁住事实，发货与退款互斥；锁顺序固定为订单、库存、钱包。
const HGMallBuyerOrderForUpdateSQL = `SELECT f.sku_id,f.quantity,f.debit_transaction_id,f.payment_id,t.payable_amount,t.trade_status,f.shipping_status,f.merchant_id FROM mall_fulfillment f JOIN trade_order t ON t.trade_order_id=f.trade_order_id WHERE f.trade_order_id=? AND f.user_id=? FOR UPDATE`
const HGMallSellerOrderForUpdateSQL = `SELECT t.payable_amount,t.trade_status,f.shipping_status,f.user_id,f.carrier,f.tracking_number FROM mall_fulfillment f JOIN mall_merchant m ON m.merchant_id=f.merchant_id AND m.status=1 JOIN trade_order t ON t.trade_order_id=f.trade_order_id WHERE f.trade_order_id=? AND m.user_id=? FOR UPDATE`

// 已发货订单确认收货后履行历史结算义务；即使店铺后来停用也不能遗失原卖家身份。
const HGMallMerchantUserSQL = `SELECT user_id FROM mall_merchant WHERE merchant_id=?`
const HGMallShipSQL = `UPDATE mall_fulfillment SET shipping_status=1,carrier=?,tracking_number=?,shipped_at=UTC_TIMESTAMP(3) WHERE trade_order_id=? AND shipping_status=0`
const HGMallTradeShippedSQL = `UPDATE trade_order SET trade_status=3 WHERE trade_order_id=? AND trade_status=2`
const HGMallReceiveSQL = `UPDATE mall_fulfillment SET shipping_status=2,received_at=UTC_TIMESTAMP(3) WHERE trade_order_id=? AND shipping_status=1`
const HGMallTradeReceivedSQL = `UPDATE trade_order SET trade_status=4,completed_at=UTC_TIMESTAMP(3) WHERE trade_order_id=? AND trade_status=3`
const HGMallRefundInsertSQL = `INSERT INTO refund_order(refund_order_id,trade_order_id,payment_order_id,user_id,request_id,request_hash,currency,amount,refund_status,reason,completed_at) VALUES(?,?,?,?,?,?,'MLC_COIN',?,2,'unshipped_cancel',UTC_TIMESTAMP(3))`
const HGMallRefundByRequestSQL = `SELECT trade_order_id,request_hash,refund_status FROM refund_order WHERE user_id=? AND request_id=? LIMIT 1`
const HGMallTradeRefundSQL = `UPDATE trade_order SET trade_status=7,cancelled_at=UTC_TIMESTAMP(3) WHERE trade_order_id=? AND trade_status=2`
const HGMallFulfillmentRefundSQL = `UPDATE mall_fulfillment SET shipping_status=3 WHERE trade_order_id=? AND shipping_status=0`

package SQLQueriesPackage

// SIMULATED 支付只能由严格 debug+开关+验签事件使用；单订单索引点查与同事务 CAS，严禁接入正式渠道。
const (
	HGCommerceSimOrderLockSQL       = `SELECT user_id,product_id,sku_code,quantity,status,expire_at,total_cent,merchant_id FROM commerce_order WHERE trade_order_id=? FOR UPDATE`
	HGCommerceSimPaymentGetSQL      = `SELECT payment_id,user_id,amount,payment_status FROM payment_order WHERE trade_order_id=? AND payment_type='SIMULATED' AND channel='SIMULATED' ORDER BY id DESC LIMIT 1`
	HGCommerceSimPaymentInsertSQL   = `INSERT INTO payment_order(payment_id,trade_order_id,user_id,request_id,request_hash,payment_type,currency,amount,payment_status,merchant_id,channel,expire_at) VALUES(?,?,?,?,?,'SIMULATED','CNY',?,1,?,'SIMULATED',?)`
	HGCommerceSimPaymentSuccessSQL  = `UPDATE payment_order SET payment_status=2,paid_at=UTC_TIMESTAMP(3),external_payment_id=? WHERE payment_id=? AND trade_order_id=? AND currency='CNY' AND payment_type='SIMULATED' AND channel='SIMULATED' AND amount=? AND payment_status=1`
	HGCommerceSimEventReadSQL       = `SELECT payload_hash,result FROM commerce_simulated_payment_event WHERE event_id=?`
	HGCommerceSimEventInsertSQL     = `INSERT INTO commerce_simulated_payment_event(event_id,payment_id,trade_order_id,external_payment_id,amount_cent,payload_hash,result) VALUES(?,?,?,?,?,?,?)`
	HGCommerceSimReservationSellSQL = `UPDATE commerce_inventory_reservation SET status=3 WHERE trade_order_id=? AND status=1`
	HGCommerceSimStockSellSQL       = `UPDATE commerce_inventory SET reserved=reserved-?,sold=sold+?,version=version+1 WHERE product_id=? AND sku_code=? AND reserved>=?`
	HGCommerceSimOrderPaidSQL       = `UPDATE commerce_order SET status=2 WHERE trade_order_id=? AND status=1`
	HGCommerceSimTradePaidSQL       = `UPDATE trade_order SET trade_status=2,paid_at=UTC_TIMESTAMP(3) WHERE trade_order_id=? AND currency='CNY' AND trade_status=1`
	// 已取消或过期时仍记录实际模拟收款并创建模拟原路退款事实，不推进订单至已支付。
	HGCommerceSimRefundCreateSQL = `INSERT INTO refund_order(refund_order_id,trade_order_id,payment_order_id,user_id,request_id,request_hash,currency,amount,refund_status,reason,completed_at) VALUES(?,?,?,?,?,?,'CNY',?,2,?,UTC_TIMESTAMP(3))`
	HGCommerceSimOrderRefundSQL  = `UPDATE commerce_order SET status=6 WHERE trade_order_id=? AND status=2`
	HGCommerceSimTradeRefundSQL  = `UPDATE trade_order SET trade_status=7 WHERE trade_order_id=? AND currency='CNY' AND trade_status=2`
	HGCommerceSimRefundReadSQL   = `SELECT refund_order_id,refund_status FROM refund_order WHERE trade_order_id=? AND request_id=?`
	// 严格检查支付事实来自 SIMULATED，不以客户端声明或者现有 ALIPAY/WECHAT 模拟成功作为证明。
	HGCommerceSimShipLockSQL  = `SELECT o.status FROM commerce_order o JOIN mall_merchant m ON m.merchant_id=o.merchant_id AND m.user_id=? AND m.status=1 WHERE o.trade_order_id=? FOR UPDATE`
	HGCommerceSimShipProofSQL = `SELECT 1 FROM trade_order t JOIN payment_order pay ON pay.trade_order_id=t.trade_order_id AND pay.currency='CNY' AND pay.payment_type='SIMULATED' AND pay.channel='SIMULATED' AND pay.payment_status=2 AND pay.external_payment_id IS NOT NULL WHERE t.trade_order_id=? AND t.currency='CNY' AND t.trade_status=2 LIMIT 1`
	HGCommerceSimReconcileSQL = `SELECT o.status,t.trade_status,COALESCE(p.payment_status,0),r.status,COALESCE(e.result,''),COALESCE(f.refund_status,0) FROM commerce_order o JOIN trade_order t ON t.trade_order_id=o.trade_order_id JOIN commerce_inventory_reservation r ON r.trade_order_id=o.trade_order_id LEFT JOIN payment_order p ON p.trade_order_id=o.trade_order_id AND p.payment_type='SIMULATED' AND p.channel='SIMULATED' LEFT JOIN commerce_simulated_payment_event e ON e.trade_order_id=o.trade_order_id LEFT JOIN refund_order f ON f.trade_order_id=o.trade_order_id AND f.request_id=CONCAT('SIMREF_',o.trade_order_id) WHERE o.trade_order_id=? LIMIT 1`
)

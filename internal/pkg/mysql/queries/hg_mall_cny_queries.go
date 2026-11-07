package SQLQueriesPackage

// 人民币目录使用 (status,id) 游标及 LIMIT 21；店铺状态走 mall_merchant 主键，未审核商家不可展示。
const HGCNYListSQL = `SELECT p.id,p.product_id,p.merchant_id,p.title,p.description,p.image_url,p.sku_id,p.sku_title,p.price_fen,p.group_price_fen,p.group_members,i.available,m.shop_name
FROM hg_mall_cny_product p JOIN hg_mall_cny_inventory i ON i.sku_id=p.sku_id JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
WHERE p.status=1 AND p.id<? ORDER BY p.id DESC LIMIT ?`
const HGCNYDetailSQL = `SELECT p.product_id,p.merchant_id,p.title,p.description,p.image_url,p.sku_id,p.sku_title,p.price_fen,p.group_price_fen,p.group_members,i.available,m.shop_name
FROM hg_mall_cny_product p JOIN hg_mall_cny_inventory i ON i.sku_id=p.sku_id JOIN mall_merchant m ON m.merchant_id=p.merchant_id AND m.status=1
WHERE p.product_id=? AND p.status=1 LIMIT 1`

// 发布在短事务内锁定已审核商家；商品和库存必须原子提交。
const HGCNYMerchantSQL = `SELECT merchant_id FROM mall_merchant WHERE user_id=? AND status=1 FOR UPDATE`
const HGCNYProductInsertSQL = `INSERT INTO hg_mall_cny_product(product_id,merchant_id,title,description,image_url,sku_id,sku_title,price_fen,group_price_fen,group_members) VALUES(?,?,?,?,?,?,?,?,?,?)`
const HGCNYInventoryInsertSQL = `INSERT INTO hg_mall_cny_inventory(sku_id,available) VALUES(?,?)`
const HGCNYMerchantProductSQL = `SELECT 1 FROM hg_mall_cny_product WHERE product_id=? AND merchant_id=? AND status=1 LIMIT 1`
const HGCNYMerchantOwnerSQL = `SELECT 1 FROM mall_merchant WHERE merchant_id=? AND user_id=? AND status=1 LIMIT 1`
const HGCNYCouponInsertSQL = `INSERT INTO hg_mall_cny_coupon(coupon_id,product_id,threshold_fen,discount_fen,remaining,expires_at) VALUES(?,?,?,?,?,?)`

// 商品券按 (product_id,expires_at) 有界取最近 20 张，不读取个人领取情况。
const HGCNYCouponListSQL = `SELECT coupon_id,product_id,threshold_fen,discount_fen,remaining,expires_at FROM hg_mall_cny_coupon WHERE product_id=? AND expires_at>UTC_TIMESTAMP(3) AND remaining>0 ORDER BY expires_at ASC,coupon_id ASC LIMIT 20`
const HGCNYCouponClaimSQL = `UPDATE hg_mall_cny_coupon SET remaining=remaining-1 WHERE coupon_id=? AND product_id=? AND remaining>0 AND expires_at>UTC_TIMESTAMP(3)`
const HGCNYUserCouponInsertSQL = `INSERT INTO hg_mall_cny_user_coupon(user_id,coupon_id) VALUES(?,?)`

// 单买报价必须验证已领券、适用商品、有效期及门槛；用户 ID 来自 JWT。
const HGCNYOwnedCouponSQL = `SELECT c.threshold_fen,c.discount_fen FROM hg_mall_cny_user_coupon u JOIN hg_mall_cny_coupon c ON c.coupon_id=u.coupon_id WHERE u.user_id=? AND u.coupon_id=? AND u.status=0 AND c.product_id=? AND c.expires_at>UTC_TIMESTAMP(3) LIMIT 1`
const HGCNYLockCouponSQL = `UPDATE hg_mall_cny_user_coupon SET status=1,order_id=? WHERE user_id=? AND coupon_id=? AND status=0 AND order_id IS NULL`
const HGCNYCouponSpendSQL = `UPDATE hg_mall_cny_user_coupon SET status=2 WHERE user_id=? AND coupon_id=? AND order_id=? AND status=1`
const HGCNYCouponReleaseSQL = `UPDATE hg_mall_cny_user_coupon SET status=0,order_id=NULL WHERE user_id=? AND coupon_id=? AND order_id=? AND status=1`

// 拼团同商品最多读取 20 个招募实例，(product_id,status,expires_at) 限制范围；不是全表扫描。
const HGCNYGroupListSQL = `SELECT group_id,product_id,target_members,paid_members,reserved_members,status,expires_at FROM hg_mall_cny_group WHERE product_id=? AND status=1 AND expires_at>UTC_TIMESTAMP(3) ORDER BY expires_at ASC,group_id ASC LIMIT 20`
const HGCNYGroupSQL = `SELECT group_id,product_id,target_members,paid_members,reserved_members,status,expires_at FROM hg_mall_cny_group WHERE group_id=? LIMIT 1`
const HGCNYGroupLockSQL = `SELECT group_id,product_id,target_members,paid_members,reserved_members,status,expires_at FROM hg_mall_cny_group WHERE group_id=? FOR UPDATE`
const HGCNYGroupInsertSQL = `INSERT INTO hg_mall_cny_group(group_id,product_id,creator_id,target_members,reserved_members,status,expires_at) VALUES(?,?,?,?,1,0,?)`
const HGCNYGroupReserveSQL = `UPDATE hg_mall_cny_group SET reserved_members=reserved_members+1 WHERE group_id=? AND product_id=? AND status=1 AND expires_at>UTC_TIMESTAMP(3) AND paid_members+reserved_members<target_members`

// MySQL 单表 UPDATE 赋值从左到右；status 比较的是本句已递增的 paid_members，不能再 +1，否则提前成团。
const HGCNYGroupPaidSQL = `UPDATE hg_mall_cny_group SET reserved_members=reserved_members-1,paid_members=paid_members+1,status=CASE WHEN paid_members=target_members THEN 2 ELSE 1 END,expires_at=CASE WHEN status=1 AND paid_members=1 THEN UTC_TIMESTAMP(3)+INTERVAL 1 DAY ELSE expires_at END WHERE group_id=? AND reserved_members>0 AND status IN (0,1) AND expires_at>UTC_TIMESTAMP(3)`
const HGCNYGroupReleaseSQL = `UPDATE hg_mall_cny_group SET reserved_members=reserved_members-1,status=CASE WHEN status=0 THEN 4 ELSE status END WHERE group_id=? AND reserved_members>0 AND status IN (0,1,3,4,5)`

// 同一次购买先查 (user_id,request_id) 唯一索引；命中时比较哈希，不重复预留库存或券。
const HGCNYOrderReplaySQL = `SELECT order_id,product_id,title,quantity,purchase_type,COALESCE(group_id,''),COALESCE(coupon_id,''),total_fen,discount_fen,payable_fen,status,expires_at,request_hash FROM hg_mall_cny_order WHERE user_id=? AND request_id=? LIMIT 1`
const HGCNYOrderInsertSQL = `INSERT INTO hg_mall_cny_order(order_id,user_id,request_id,request_hash,product_id,sku_id,title,quantity,purchase_type,group_id,coupon_id,total_fen,discount_fen,payable_fen,address_snapshot,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

// (status,expires_at,id) 超时任务应按有界批次；库存预留使用 sku 主键条件更新，不能先查后无条件写。
const HGCNYReserveStockSQL = `UPDATE hg_mall_cny_inventory SET available=available-?,reserved=reserved+? WHERE sku_id=? AND available>=?`
const HGCNYCommitStockSQL = `UPDATE hg_mall_cny_inventory SET reserved=reserved-?,sold=sold+? WHERE sku_id=? AND reserved>=?`
const HGCNYReleaseStockSQL = `UPDATE hg_mall_cny_inventory SET available=available+?,reserved=reserved-? WHERE sku_id=? AND reserved>=?`
const HGCNYRestockSQL = `UPDATE hg_mall_cny_inventory SET available=available+?,sold=sold-? WHERE sku_id=? AND sold>=?`

// 本人订单及支付/退款短事务按订单唯一键锁行；不允许客户端传入金额。
const HGCNYOrderLockSQL = `SELECT order_id,product_id,title,quantity,purchase_type,COALESCE(group_id,''),COALESCE(coupon_id,''),total_fen,discount_fen,payable_fen,status,expires_at,sku_id FROM hg_mall_cny_order WHERE order_id=? AND user_id=? FOR UPDATE`
const HGCNYOrderOwnerSQL = `SELECT order_id,product_id,title,quantity,purchase_type,COALESCE(group_id,''),COALESCE(coupon_id,''),total_fen,discount_fen,payable_fen,status,expires_at FROM hg_mall_cny_order WHERE order_id=? AND user_id=? LIMIT 1`
const HGCNYOrdersSQL = `SELECT id,order_id,product_id,title,quantity,purchase_type,COALESCE(group_id,''),COALESCE(coupon_id,''),total_fen,discount_fen,payable_fen,status,expires_at FROM hg_mall_cny_order WHERE user_id=? AND id<? ORDER BY id DESC LIMIT 21`
const HGCNYOrderPaidSQL = `UPDATE hg_mall_cny_order SET status=?,paid_at=UTC_TIMESTAMP(3) WHERE order_id=? AND user_id=? AND status=0 AND expires_at>UTC_TIMESTAMP(3)`
const HGCNYOrderCancelSQL = `UPDATE hg_mall_cny_order SET status=3 WHERE order_id=? AND user_id=? AND status=0`
const HGCNYOrderRefundSQL = `UPDATE hg_mall_cny_order SET status=4 WHERE order_id=? AND user_id=? AND status IN (1,2)`

// 拼团最多 10 个不同用户，成团后只更新同 group_id 的已付待成团订单，不触碰其他商家。
const HGCNYGroupOrdersReadySQL = `UPDATE hg_mall_cny_order SET status=1 WHERE group_id=? AND status=2`
const HGCNYPaymentInsertSQL = `INSERT INTO hg_mall_cny_payment(order_id,payment_id,user_id,amount_fen) VALUES(?,?,?,?)`
const HGCNYPaymentAmountSQL = `SELECT amount_fen FROM hg_mall_cny_payment WHERE order_id=? AND user_id=? LIMIT 1`
const HGCNYRefundInsertSQL = `INSERT INTO hg_mall_cny_refund(order_id,refund_id,user_id,amount_fen) VALUES(?,?,?,?)`
const HGCNYGroupFailSQL = `UPDATE hg_mall_cny_group SET status=3 WHERE group_id=? AND status=1 AND expires_at<=UTC_TIMESTAMP(3)`

// 定时任务每轮只处理最多 20 笔过期待支付及 10 个过期/待结算拼团；根据唯一 ID 重放幂等。
const HGCNYExpiredOrderBatchSQL = `SELECT order_id,user_id FROM hg_mall_cny_order WHERE status=0 AND expires_at<=UTC_TIMESTAMP(3) ORDER BY expires_at,id LIMIT 20`
const HGCNYExpiredGroupBatchSQL = `SELECT group_id FROM hg_mall_cny_group WHERE status=1 AND expires_at<=UTC_TIMESTAMP(3) ORDER BY expires_at,group_id LIMIT 10`
const HGCNYFailedGroupBatchSQL = `SELECT group_id FROM hg_mall_cny_group WHERE status=3 ORDER BY expires_at,group_id LIMIT 10`
const HGCNYGroupOpenOrdersSQL = `SELECT order_id,user_id,status FROM hg_mall_cny_order WHERE group_id=? AND status IN (0,2) ORDER BY id LIMIT 11`
const HGCNYGroupSettledSQL = `UPDATE hg_mall_cny_group SET status=5 WHERE group_id=? AND status=3 AND NOT EXISTS (SELECT 1 FROM hg_mall_cny_order WHERE group_id=? AND status IN (0,2))`

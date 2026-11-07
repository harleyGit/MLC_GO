package SQLQueriesPackage

// 以下 SQL 仅供显式开启、本机隔离 debug 数据库的 CNY 集成夹具使用；均用业务 ID 精确限制清理范围。
const HGCNYFixtureMerchantInsertSQL = `INSERT INTO mall_merchant(merchant_id,user_id,shop_name,status) VALUES(?,?,?,1)`
const HGCNYFixtureExpireGroupSQL = `UPDATE hg_mall_cny_group SET expires_at=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND WHERE group_id=? AND status=1`
const HGCNYFixtureExpireOrderSQL = `UPDATE hg_mall_cny_order SET expires_at=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND WHERE order_id=? AND status=0`
const HGCNYFixtureInventorySQL = `SELECT available,reserved,sold FROM hg_mall_cny_inventory WHERE sku_id=?`
const HGCNYFixtureCouponStatusSQL = `SELECT status FROM hg_mall_cny_user_coupon WHERE user_id=? AND coupon_id=?`
const HGCNYFixtureDeleteRefundSQL = `DELETE FROM hg_mall_cny_refund WHERE order_id=?`
const HGCNYFixtureDeletePaymentSQL = `DELETE FROM hg_mall_cny_payment WHERE order_id=?`
const HGCNYFixtureDeleteUserCouponSQL = `DELETE FROM hg_mall_cny_user_coupon WHERE user_id=? AND coupon_id=?`
const HGCNYFixtureDeleteOrderSQL = `DELETE FROM hg_mall_cny_order WHERE order_id=?`
const HGCNYFixtureDeleteGroupSQL = `DELETE FROM hg_mall_cny_group WHERE group_id=?`
const HGCNYFixtureDeleteCouponSQL = `DELETE FROM hg_mall_cny_coupon WHERE coupon_id=?`
const HGCNYFixtureDeleteInventorySQL = `DELETE FROM hg_mall_cny_inventory WHERE sku_id=?`
const HGCNYFixtureDeleteProductSQL = `DELETE FROM hg_mall_cny_product WHERE product_id=?`
const HGCNYFixtureDeleteMerchantSQL = `DELETE FROM mall_merchant WHERE merchant_id=? AND user_id=?`

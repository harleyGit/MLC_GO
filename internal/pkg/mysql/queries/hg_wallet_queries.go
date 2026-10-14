package SQLQueriesPackage

// 钱包目录按(is_deleted,status,id)索引向后扫描固定候选窗口，不做COUNT/OFFSET。
// 时间过滤由业务层对窗口执行，防止大量过期目录导致单次请求无界扫描；空页仍可续页。
const HGWalletSKUWindowSQL = `SELECT id, sku_id, title, currency, pay_amount, coin_amount, bonus_coin, total_coin, start_time, end_time
	FROM payment_recharge_sku FORCE INDEX (idx_wallet_sku_visible_id)
	WHERE is_deleted = 0 AND status = 1 AND id > ? ORDER BY id LIMIT 201`

// 创建订单短事务只共享锁定一个档位，避免运维修改和快照交错；有效期采用服务端UTC时间。
const HGWalletSKUForShareSQL = `SELECT id, sku_id, title, currency, pay_amount, coin_amount, bonus_coin, total_coin, start_time, end_time
	FROM payment_recharge_sku WHERE sku_id = ? AND is_deleted = 0 AND status = 1
	AND start_time <= ? AND (end_time IS NULL OR end_time > ?) FOR SHARE`

// 展示名按users.user_id唯一索引点查，只取昵称，不读取手机号邮箱等凭据或隐私字段。
const HGWalletDisplayNameSQL = `SELECT COALESCE(NULLIF(nickname, ''), NULLIF(user_name, ''), '用户') FROM users WHERE user_id = ?`

// 创建仅插入快照并绑定模式；唯一键(user_id,request_id)保证并发幂等，此步骤不写资产。
const HGWalletInsertOrderSQL = `INSERT INTO payment_recharge_order
	(recharge_order_id, trade_order_id, user_id, request_id, request_hash, sku_id, display_name, sku_snapshot_title, description, currency, pay_amount, coin_amount, bonus_coin, total_coin, created_at, expire_at, payment_mode, snapshot_source, order_status)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'wallet', 1)`

// 与充值单同事务创建；交易ID沿用充值orderId，跨业务requestId碰撞不得覆盖。
const HGWalletInsertTradeSQL = `INSERT INTO trade_order
	(trade_order_id, user_id, request_id, request_hash, order_type, title, currency, total_amount, payable_amount, trade_status, source, expire_at, created_at)
	VALUES (?, ?, ?, ?, 'RECHARGE', ?, ?, ?, ?, 1, 'WALLET', ?, ?)`

// 点查命中订单唯一键或用户幂等唯一键；owner条件始终在SQL中执行，不泄露其他用户订单。
const hgWalletOrderColumns = `SELECT r.recharge_order_id, r.user_id, r.request_id, r.sku_id, r.display_name, r.sku_snapshot_title, r.description, r.currency, r.pay_amount, r.total_coin, r.created_at, r.expire_at, r.payment_mode,
	CASE WHEN r.order_status = 3 THEN 'paid' ELSE 'pending' END, r.paid_at, r.paid_transaction_id, r.balance_after, r.request_hash,
	COALESCE(t.trade_order_id = r.recharge_order_id AND BINARY t.user_id = BINARY r.user_id AND t.request_id = r.request_id
	AND t.request_hash = r.request_hash AND t.order_type = 'RECHARGE' AND t.source = 'WALLET'
	AND BINARY t.title = BINARY r.sku_snapshot_title AND t.currency = r.currency AND t.total_amount = r.pay_amount
	AND t.payable_amount = r.pay_amount AND t.discount_amount = 0 AND t.expire_at = r.expire_at AND t.created_at = r.created_at
	AND t.paid_at <=> r.paid_at AND t.completed_at <=> r.completed_at
	AND r.payment_status = 0 AND ((r.order_status = 1 AND t.trade_status = 1 AND r.paid_at IS NULL AND r.completed_at IS NULL AND r.paid_transaction_id IS NULL AND r.balance_after IS NULL)
	OR (r.order_status = 3 AND t.trade_status = 4 AND r.payment_mode = 'platform_debug' AND r.paid_at IS NOT NULL AND r.completed_at = r.paid_at AND r.paid_transaction_id > 0 AND r.balance_after IS NOT NULL)), 0)
	FROM payment_recharge_order r LEFT JOIN trade_order t ON t.trade_order_id = r.trade_order_id `
const HGWalletOrderByOwnerSQL = hgWalletOrderColumns + `WHERE r.recharge_order_id = ? AND BINARY r.user_id = BINARY ? AND r.snapshot_source IN ('wallet', 'wallet_legacy')`
const HGWalletOrderByRequestSQL = hgWalletOrderColumns + `WHERE r.user_id = ? AND r.request_id = ?`

// 按订单唯一索引锁一行，固定顺序为订单后coin钱包；与coin入账同短事务，无外部IO。
const HGWalletOrderForPaySQL = HGWalletOrderByOwnerSQL + ` FOR UPDATE`
const HGWalletMarkPaidSQL = `UPDATE payment_recharge_order SET order_status = 3, paid_at = ?, completed_at = ?, paid_transaction_id = ?, balance_after = ? WHERE recharge_order_id = ? AND user_id = ? AND order_status = 1 AND payment_status = 0 AND payment_mode = 'platform_debug'`

// debug完成仅表达本地资产到账，payment_status保持INIT，不虚构真实支付成功。
const HGWalletCompleteTradeSQL = `UPDATE trade_order SET trade_status = 4, paid_at = ?, completed_at = ? WHERE trade_order_id = ? AND user_id = ? AND trade_status = 1 AND order_type = 'RECHARGE' AND source = 'WALLET'`

// 手动迁移仅接受闭区间，high-low < 500；PRIMARY范围扫描最多500行，必须停旧写入。
// 旧SKU是API限定的ASCII标识，JSON编码与Go的规范化哈希一致。
const HGWalletLegacyRangeSQL = `SELECT order_id, user_id, request_id, sku_id, display_name, title, description, currency, pay_amount, total_coin, created_at, expires_at, payment_mode, status, paid_at, paid_transaction_id, balance_after,
	SHA2(CONCAT('{"skuId":"', sku_id, '"}'), 256),
	COALESCE((status = 'pending' AND paid_at IS NULL AND paid_transaction_id IS NULL AND balance_after IS NULL)
	OR (status = 'paid' AND payment_mode = 'platform_debug' AND paid_at IS NOT NULL AND paid_transaction_id > 0 AND balance_after IS NOT NULL), 0)
	FROM wallet_recharge_orders FORCE INDEX (PRIMARY) WHERE id >= ? AND id <= ? ORDER BY id LIMIT 500`

// 每行唯一键插入，不忽略冲突，不触碰coin、Outbox或支付表；整个区间一个事务。
const HGWalletInsertLegacyOrderSQL = `INSERT INTO payment_recharge_order
	(recharge_order_id, trade_order_id, user_id, request_id, request_hash, sku_id, display_name, sku_snapshot_title, description, currency, pay_amount, coin_amount, bonus_coin, total_coin, created_at, expire_at, payment_mode, snapshot_source, order_status, paid_at, completed_at, paid_transaction_id, balance_after)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, ?, ?, ?, ?, 'wallet_legacy', ?, ?, ?, ?, ?)`

// 恢复仅接受同来源且无伪造拆分的旧单，再由Go逐字段校验快照及关联交易一致性。
const HGWalletLegacyByOwnerSQL = HGWalletOrderByOwnerSQL + ` AND r.snapshot_source = 'wallet_legacy' AND r.coin_amount IS NULL AND r.bonus_coin IS NULL`

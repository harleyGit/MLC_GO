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

// 订单仅插入一次；唯一键(user_id,request_id)保证并发幂等，不进行任何资产写入。
const HGWalletInsertOrderSQL = `INSERT INTO wallet_recharge_orders
	(order_id, user_id, request_id, sku_id, display_name, title, description, currency, pay_amount, total_coin, created_at, expires_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// 点查命中订单唯一键或用户幂等唯一键；owner条件始终在SQL中执行，不泄露其他用户订单。
const hgWalletOrderColumns = `SELECT order_id, user_id, request_id, sku_id, display_name, title, description, currency, pay_amount, total_coin, created_at, expires_at FROM wallet_recharge_orders `
const HGWalletOrderByOwnerSQL = hgWalletOrderColumns + `WHERE order_id = ? AND user_id = ?`
const HGWalletOrderByRequestSQL = hgWalletOrderColumns + `WHERE user_id = ? AND request_id = ?`

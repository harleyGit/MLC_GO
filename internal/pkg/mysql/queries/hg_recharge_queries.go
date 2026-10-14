package SQLQueriesPackage

// 充值记录只读统一表；依赖 idx_payment_recharge_order_user_created_id
// (user_id, created_at DESC, id DESC)。普通等值保留索引范围，BINARY 排除排序规则折叠的其他用户。
// 每次最多 51 行，不联表、不 COUNT/OFFSET；生产索引须由发布迁移保证。
const hgRechargeColumns = `SELECT id, recharge_order_id, sku_snapshot_title, currency, pay_amount, total_coin,
	order_status, payment_mode, created_at, expire_at, paid_at
	FROM payment_recharge_order FORCE INDEX (idx_payment_recharge_order_user_created_id)
	WHERE user_id = ? AND BINARY user_id = BINARY ?`

const HGRechargeFirstPageSQL = hgRechargeColumns + ` ORDER BY created_at DESC, id DESC LIMIT ?`

// 展开复合游标条件，保持索引范围扫描与稳定倒序；不以可变状态过滤记录。
const HGRechargeNextPageSQL = hgRechargeColumns + ` AND (created_at < ? OR (created_at = ? AND id < ?))
	ORDER BY created_at DESC, id DESC LIMIT ?`

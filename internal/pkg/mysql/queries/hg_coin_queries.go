package SQLQueriesPackage

const (
	// 资产查询均以 user_id/request_id 唯一键、钱包主键或 lot 复合索引访问；禁止在在线事务中全表扫描。
	InsertCoinRequestSQL = `INSERT IGNORE INTO coin_asset_requests
		(user_id, request_id, operation, command_hash, status) VALUES (?, ?, ?, ?, 'processing')`
	SelectCoinRequestSQL = `SELECT operation, command_hash, status, transaction_id, balance_after
		FROM coin_asset_requests WHERE user_id = ? AND request_id = ?`
	CompleteCoinRequestSQL = `UPDATE coin_asset_requests SET status = 'completed', transaction_id = ?, balance_after = ?, updated_at = NOW()
		WHERE user_id = ? AND request_id = ? AND status = 'processing'`

	// 新旧两套投币命令只读汇总，保证 migration 12 升级后的单视频累计额度不会清零。
	SelectCoinBusinessDebitTotalSQL = `SELECT
		(SELECT COALESCE(SUM(amount), 0) FROM coin_asset_transactions
		 WHERE user_id = ? AND operation = 'debit' AND business_type = ? AND business_key = ?)
		+
		(SELECT CASE WHEN ? = 'video_coin' THEN COALESCE(SUM(quantity), 0) ELSE 0 END
		 FROM user_coin_commands WHERE user_id = ? AND submission_id = ? AND status = 'completed')`
	SelectLegacyCoinCommandSQL = `SELECT submission_id, quantity, status FROM user_coin_commands
		WHERE user_id = ? AND request_id = ?`
	// FEFO 只锁定固定上限的最早到期 lot；Service 同步限制单次最多消费 1000 枚。
	SelectCoinLotsForDebitSQL = `SELECT id, remaining_amount, expires_at FROM coin_asset_lots FORCE INDEX (idx_coin_lot_fefo)
		WHERE user_id = ? AND remaining_amount > 0 AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY expires_sort, id LIMIT 1000 FOR UPDATE`
	UpdateCoinLotRemainingSQL = `UPDATE coin_asset_lots SET remaining_amount = ?, updated_at = NOW() WHERE id = ?`
	InsertCoinTransactionSQL  = `INSERT INTO coin_asset_transactions
		(user_id, request_id, operation, amount, signed_delta, balance_after, reason, business_type, business_key, reference_transaction_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0))`
	InsertCoinLotSQL = `INSERT INTO coin_asset_lots
		(user_id, source_transaction_id, original_amount, remaining_amount, expires_at) VALUES (?, ?, ?, ?, ?)`
	InsertCoinAllocationSQL = `INSERT INTO coin_asset_allocations (transaction_id, lot_id, amount, allocation_type) VALUES (?, ?, ?, ?)`
	CreditCoinWalletSQL     = `UPDATE user_coin_wallets SET balance = balance + ?, updated_at = NOW()
		WHERE user_id = ? AND balance <= ?`
	SelectCoinDebitForRefundSQL = `SELECT debit.id, debit.amount, COALESCE(SUM(refund.amount), 0)
		FROM coin_asset_transactions debit
		LEFT JOIN coin_asset_transactions refund ON refund.reference_transaction_id = debit.id AND refund.operation = 'refund'
		WHERE debit.id = ? AND debit.user_id = ? AND debit.operation = 'debit'
		GROUP BY debit.id, debit.amount FOR UPDATE`

	// 到期候选按 expires_at,id 游标顺序读取固定批次，具体扣减在逐 lot 短事务中再次校验并加锁。
	SelectExpiredCoinLotsSQL = `SELECT id, user_id FROM coin_asset_lots FORCE INDEX (idx_coin_lot_expiration)
		WHERE expires_at <= ? AND remaining_amount > 0 ORDER BY expires_at, id LIMIT ?`
	SelectExpiredCoinLotForUpdateSQL = `SELECT remaining_amount FROM coin_asset_lots WHERE id = ? AND user_id = ? AND expires_at <= ? FOR UPDATE`
	SelectCoinWalletSQL              = `SELECT balance FROM user_coin_wallets WHERE user_id = ?`
	// 运维流水严格按 user_id 和 created_at,id 复合游标查询，命中 idx_coin_transaction_user_created；多取一条判断 hasMore，不执行 COUNT/OFFSET。
	SelectCoinTransactionsFirstSQL = `SELECT id, user_id, request_id, operation, amount, signed_delta, balance_after, reason, business_type, business_key, reference_transaction_id, created_at
		FROM coin_asset_transactions FORCE INDEX (idx_coin_transaction_user_created)
		WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`
	SelectCoinTransactionsByCursorSQL = `SELECT id, user_id, request_id, operation, amount, signed_delta, balance_after, reason, business_type, business_key, reference_transaction_id, created_at
		FROM coin_asset_transactions FORCE INDEX (idx_coin_transaction_user_created)
		WHERE user_id = ? AND (created_at < ? OR (created_at = ? AND id < ?))
		ORDER BY created_at DESC, id DESC LIMIT ?`
	// 历史钱包初始化命中 users 主键并使用 keyset cursor，避免 OFFSET 深分页。
	SelectUsersAfterCoinCursorSQL      = `SELECT id, user_id FROM users WHERE id > ? AND user_id IS NOT NULL ORDER BY id LIMIT ?`
	SelectCoinInitializerCheckpointSQL = `SELECT cursor_value FROM coin_job_checkpoints WHERE job_name = 'wallet_initializer'`
	UpsertCoinInitializerCheckpointSQL = `INSERT INTO coin_job_checkpoints (job_name, cursor_value) VALUES ('wallet_initializer', ?)
		ON DUPLICATE KEY UPDATE cursor_value = GREATEST(cursor_value, VALUES(cursor_value)), updated_at = NOW()`
	SelectCoinReconciliationPageSQL = `SELECT users.id, wallet.user_id, wallet.balance,
		(SELECT COALESCE(SUM(lot.remaining_amount), 0) FROM coin_asset_lots lot FORCE INDEX (idx_coin_lot_fefo) WHERE lot.user_id = wallet.user_id)
		FROM users JOIN user_coin_wallets wallet ON wallet.user_id = users.user_id
		WHERE users.id > ? ORDER BY users.id LIMIT ?`
)

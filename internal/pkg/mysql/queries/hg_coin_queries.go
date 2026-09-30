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
		(SELECT COALESCE(SUM(amount), 0) FROM account_ledger
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
	InsertCoinTransactionSQL  = `INSERT INTO account_ledger
		(user_id, request_id, operation, amount, signed_delta, balance_after, reason, business_type, business_key, reference_transaction_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0))`
	InsertCoinLotSQL = `INSERT INTO coin_asset_lots
		(user_id, source_transaction_id, original_amount, remaining_amount, expires_at) VALUES (?, ?, ?, ?, ?)`
	InsertCoinAllocationSQL = `INSERT INTO coin_asset_allocations (transaction_id, lot_id, amount, allocation_type) VALUES (?, ?, ?, ?)`
	CreditCoinWalletSQL     = `UPDATE account SET balance = balance + ?, version = version + 1, updated_at = NOW()
		WHERE user_id = ? AND balance <= ?`
	SelectCoinDebitForRefundSQL = `SELECT debit.id, debit.amount, COALESCE(SUM(refund.amount), 0)
		FROM account_ledger debit
		LEFT JOIN account_ledger refund ON refund.reference_transaction_id = debit.id AND refund.operation = 'refund'
		WHERE debit.id = ? AND debit.user_id = ? AND debit.operation = 'debit'
		GROUP BY debit.id, debit.amount FOR UPDATE`

	// 到期候选按 expires_at,id 游标顺序读取固定批次，具体扣减在逐 lot 短事务中再次校验并加锁。
	SelectExpiredCoinLotsSQL = `SELECT id, user_id FROM coin_asset_lots FORCE INDEX (idx_coin_lot_expiration)
		WHERE expires_at <= ? AND remaining_amount > 0 ORDER BY expires_at, id LIMIT ?`
	SelectExpiredCoinLotForUpdateSQL = `SELECT remaining_amount FROM coin_asset_lots WHERE id = ? AND user_id = ? AND expires_at <= ? FOR UPDATE`
	SelectCoinWalletSQL              = `SELECT balance FROM account WHERE user_id = ?`
	// 账户按 user_id 主键读取；冻结尚未开放，余额语义与 coin 保持一致。
	HGSelectAccountSQL = `SELECT user_id, balance, frozen_balance, currency, account_type, version, created_at, updated_at
		FROM account WHERE user_id = ?`
	// 流水沿用复合索引与稳定排序，仅提供最近最多 100 条；完整分页沿用 coin.ListTransactions。
	HGSelectAccountLedgerSQL = `SELECT id, user_id, request_id, operation, amount, signed_delta, balance_after,
		balance_before, reason, business_type, business_key, reference_transaction_id, created_at
		FROM account_ledger FORCE INDEX (idx_coin_transaction_user_created)
		WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`
	// 仅供显式授权的隔离集成验收：按用户主键种入旧余额场景，不可用于在线资金写入。
	HGSeedAccountAcceptanceSQL = `INSERT INTO account (user_id, balance) VALUES (?, 10)
		ON DUPLICATE KEY UPDATE balance = 10, version = version + 1`
	// 验收按用户/请求唯一键与 Outbox event_key 索引检查，无全表计数。
	HGCountAccountDebitAcceptanceSQL = `SELECT COUNT(*) FROM account_ledger WHERE user_id = ? AND request_id = ? AND operation = 'debit'`
	HGCountCoinOutboxAcceptanceSQL   = `SELECT COUNT(*) FROM outbox_event WHERE event_key = ? AND event_name = ?`
	// 运维流水严格按 user_id 和 created_at,id 复合游标查询，命中 idx_coin_transaction_user_created；多取一条判断 hasMore，不执行 COUNT/OFFSET。
	SelectCoinTransactionsFirstSQL = `SELECT id, user_id, request_id, operation, amount, signed_delta, balance_after, reason, business_type, business_key, reference_transaction_id, created_at
		FROM account_ledger FORCE INDEX (idx_coin_transaction_user_created)
		WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`
	SelectCoinTransactionsByCursorSQL = `SELECT id, user_id, request_id, operation, amount, signed_delta, balance_after, reason, business_type, business_key, reference_transaction_id, created_at
		FROM account_ledger FORCE INDEX (idx_coin_transaction_user_created)
		WHERE user_id = ? AND (created_at < ? OR (created_at = ? AND id < ?))
		ORDER BY created_at DESC, id DESC LIMIT ?`
	// 历史钱包初始化命中 users 主键并使用 keyset cursor，避免 OFFSET 深分页。
	SelectUsersAfterCoinCursorSQL      = `SELECT id, user_id FROM users WHERE id > ? AND user_id IS NOT NULL ORDER BY id LIMIT ?`
	SelectCoinInitializerCheckpointSQL = `SELECT cursor_value FROM coin_job_checkpoints WHERE job_name = 'wallet_initializer'`
	UpsertCoinInitializerCheckpointSQL = `INSERT INTO coin_job_checkpoints (job_name, cursor_value) VALUES ('wallet_initializer', ?)
		ON DUPLICATE KEY UPDATE cursor_value = GREATEST(cursor_value, VALUES(cursor_value)), updated_at = NOW()`
	SelectCoinReconciliationCheckpointSQL = `SELECT cursor_value FROM coin_job_checkpoints WHERE job_name = 'wallet_reconciliation'`
	UpsertCoinReconciliationCheckpointSQL = `INSERT INTO coin_job_checkpoints (job_name, cursor_value) VALUES ('wallet_reconciliation', ?)
		ON DUPLICATE KEY UPDATE cursor_value = VALUES(cursor_value), updated_at = NOW()`
	SelectCoinConsolidationCheckpointSQL = `SELECT cursor_value FROM coin_job_checkpoints WHERE job_name = 'lot_consolidation'`
	UpsertCoinConsolidationCheckpointSQL = `INSERT INTO coin_job_checkpoints (job_name, cursor_value) VALUES ('lot_consolidation', ?)
		ON DUPLICATE KEY UPDATE cursor_value = VALUES(cursor_value), updated_at = NOW()`
	SelectCoinReconciliationPageSQL = `SELECT users.id, wallet.user_id, wallet.balance,
		(SELECT COALESCE(SUM(lot.remaining_amount), 0) FROM coin_asset_lots lot FORCE INDEX (idx_coin_lot_fefo) WHERE lot.user_id = wallet.user_id)
		FROM users JOIN account wallet ON wallet.user_id = users.user_id
		WHERE users.id > ? ORDER BY users.id LIMIT ?`
	// Consolidation discovery uses users.id keyset plus the lot small-candidate index; each selected wallet is processed in a separate short transaction.
	SelectCoinConsolidationUsersSQL = `SELECT users.id, wallet.user_id FROM users JOIN account wallet ON wallet.user_id = users.user_id
		WHERE users.id > ? AND EXISTS (SELECT 1 FROM coin_asset_lots lot FORCE INDEX (idx_coin_lot_consolidation)
		WHERE lot.user_id = wallet.user_id AND lot.remaining_amount > 0 AND lot.remaining_amount <= ?) ORDER BY users.id LIMIT ?`
	SelectCoinLotsForConsolidationSQL = `SELECT id, remaining_amount, expires_at FROM coin_asset_lots FORCE INDEX (idx_coin_lot_consolidation)
		WHERE user_id = ? AND remaining_amount > 0 AND remaining_amount <= ? ORDER BY remaining_amount, expires_sort, id LIMIT ? FOR UPDATE`
	InsertCoinConsolidationLinkSQL = `INSERT INTO coin_lot_consolidation_links
		(consolidation_transaction_id, source_lot_id, target_lot_id, amount) VALUES (?, ?, ?, ?)`
)

package SQLQueriesPackage

const (
	InsertInteractionInboxSQL = `INSERT IGNORE INTO video_interaction_inbox
		(event_id, event_name, event_key, kafka_topic, kafka_partition, kafka_offset, payload)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	SelectInteractionInboxByEventIDSQL = `SELECT event_id, event_name, event_key, kafka_topic, kafka_partition, kafka_offset, payload
		FROM video_interaction_inbox WHERE event_id = ?`
	SelectInteractionInboxByDeliverySQL = `SELECT event_id, event_name, event_key, kafka_topic, kafka_partition, kafka_offset, payload
		FROM video_interaction_inbox WHERE kafka_topic = ? AND kafka_partition = ? AND kafka_offset = ?`

	SelectVideoInteractionForUpdateSQL = `SELECT active, quantity FROM video_user_interactions
		WHERE user_id = ? AND submission_id = ? AND interaction_type = ? FOR UPDATE`
	InsertVideoInteractionSQL = `INSERT INTO video_user_interactions
		(user_id, submission_id, interaction_type, active, quantity, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW())`
	UpdateVideoInteractionSQL = `UPDATE video_user_interactions SET active = ?, quantity = ?, updated_at = NOW()
		WHERE user_id = ? AND submission_id = ? AND interaction_type = ?`
	UpsertVideoInteractionStatShardSQL = `INSERT INTO video_interaction_stat_shards
		(submission_id, shard_id, like_count, coin_count, favorite_count, share_count)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		like_count = GREATEST(0, like_count + VALUES(like_count)),
		coin_count = GREATEST(0, coin_count + VALUES(coin_count)),
		favorite_count = GREATEST(0, favorite_count + VALUES(favorite_count)),
		share_count = GREATEST(0, share_count + VALUES(share_count)), updated_at = NOW()`

	SelectFollowForUpdateSQL = `SELECT active FROM user_follow_relations
		WHERE follower_id = ? AND followee_id = ? FOR UPDATE`
	InsertFollowSQL = `INSERT INTO user_follow_relations (follower_id, followee_id, active, updated_at)
		VALUES (?, ?, ?, NOW())`
	UpdateFollowSQL = `UPDATE user_follow_relations SET active = ?, updated_at = NOW()
		WHERE follower_id = ? AND followee_id = ?`
	UpsertFollowStatShardSQL = `INSERT INTO user_follow_stat_shards (user_id, shard_id, follower_count)
		VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE
		follower_count = GREATEST(0, follower_count + VALUES(follower_count)), updated_at = NOW()`

	InsertShareRecordSQL = `INSERT IGNORE INTO video_share_records
		(event_id, user_id, submission_id, created_at) VALUES (?, ?, ?, NOW())`

	InsertCoinCommandSQL = `INSERT IGNORE INTO user_coin_commands
		(user_id, request_id, submission_id, quantity, status) VALUES (?, ?, ?, ?, 'processing')`
	EnsureCoinWalletSQL  = `INSERT IGNORE INTO user_coin_wallets (user_id, balance) VALUES (?, 0)`
	SelectCoinCommandSQL = `SELECT submission_id, quantity, status FROM user_coin_commands
		WHERE user_id = ? AND request_id = ?`
	SelectCoinWalletForUpdateSQL   = `SELECT balance FROM user_coin_wallets WHERE user_id = ? FOR UPDATE`
	SelectCompletedCoinQuantitySQL = `SELECT COALESCE(SUM(quantity), 0) FROM user_coin_commands
		WHERE user_id = ? AND submission_id = ? AND status = 'completed' FOR UPDATE`
	DebitCoinWalletSQL = `UPDATE user_coin_wallets SET balance = balance - ?, updated_at = NOW()
		WHERE user_id = ? AND balance >= ?`
	InsertCoinLedgerSQL = `INSERT INTO user_coin_ledger
		(user_id, request_id, submission_id, delta, balance_after) VALUES (?, ?, ?, ?, ?)`
	CompleteCoinCommandSQL = `UPDATE user_coin_commands SET status = 'completed', updated_at = NOW()
		WHERE request_id = ? AND user_id = ? AND status = 'processing'`

	// Projection page queries use migration 15 bucket-first indexes and stable keysets. The stored bucket avoids runtime CRC/MOD scans.
	SelectVideoStateProjectionPageSQL = `SELECT reproject_bucket, id, user_id, submission_id, interaction_type, active, quantity, updated_at
		FROM video_user_interactions FORCE INDEX (idx_video_interaction_reproject_bucket)
		WHERE reproject_bucket >= ? AND reproject_bucket < ? AND updated_at <= ? AND
		(reproject_bucket > ? OR (reproject_bucket = ? AND updated_at > ?) OR (reproject_bucket = ? AND updated_at = ? AND id > ?))
		ORDER BY reproject_bucket, updated_at, id LIMIT ?`
	SelectFollowStateProjectionPageSQL = `SELECT reproject_bucket, id, follower_id, followee_id, active, updated_at
		FROM user_follow_relations FORCE INDEX (idx_follow_reproject_bucket)
		WHERE reproject_bucket >= ? AND reproject_bucket < ? AND updated_at <= ? AND
		(reproject_bucket > ? OR (reproject_bucket = ? AND updated_at > ?) OR (reproject_bucket = ? AND updated_at = ? AND id > ?))
		ORDER BY reproject_bucket, updated_at, id LIMIT ?`
	SelectVideoCountProjectionPageSQL = `WITH changed AS (
		SELECT reproject_bucket, updated_at, submission_id, shard_id
		FROM video_interaction_stat_shards FORCE INDEX (idx_video_count_reproject_bucket)
		WHERE reproject_bucket >= ? AND reproject_bucket < ? AND updated_at <= ? AND
		(reproject_bucket > ? OR (reproject_bucket = ? AND updated_at > ?) OR
		(reproject_bucket = ? AND updated_at = ? AND submission_id > ?) OR
		(reproject_bucket = ? AND updated_at = ? AND submission_id = ? AND shard_id > ?))
		ORDER BY reproject_bucket, updated_at, submission_id, shard_id LIMIT ?
	)
	SELECT changed.reproject_bucket, changed.updated_at, changed.submission_id, changed.shard_id,
		SUM(stats.like_count), SUM(stats.coin_count), SUM(stats.favorite_count), SUM(stats.share_count)
	FROM changed JOIN video_interaction_stat_shards stats ON stats.submission_id = changed.submission_id
	GROUP BY changed.reproject_bucket, changed.updated_at, changed.submission_id, changed.shard_id
	ORDER BY changed.reproject_bucket, changed.updated_at, changed.submission_id, changed.shard_id`
	SelectFollowCountProjectionPageSQL = `WITH changed AS (
		SELECT reproject_bucket, updated_at, user_id, shard_id
		FROM user_follow_stat_shards FORCE INDEX (idx_follow_count_reproject_bucket)
		WHERE reproject_bucket >= ? AND reproject_bucket < ? AND updated_at <= ? AND
		(reproject_bucket > ? OR (reproject_bucket = ? AND updated_at > ?) OR
		(reproject_bucket = ? AND updated_at = ? AND user_id > ?) OR
		(reproject_bucket = ? AND updated_at = ? AND user_id = ? AND shard_id > ?))
		ORDER BY reproject_bucket, updated_at, user_id, shard_id LIMIT ?
	)
	SELECT changed.reproject_bucket, changed.updated_at, changed.user_id, changed.shard_id, SUM(stats.follower_count)
	FROM changed JOIN user_follow_stat_shards stats ON stats.user_id = changed.user_id
	GROUP BY changed.reproject_bucket, changed.updated_at, changed.user_id, changed.shard_id
	ORDER BY changed.reproject_bucket, changed.updated_at, changed.user_id, changed.shard_id`
)

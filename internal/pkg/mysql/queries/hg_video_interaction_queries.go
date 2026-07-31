package SQLQueriesPackage

const (
	InsertInteractionInboxSQL = `INSERT IGNORE INTO video_interaction_inbox
		(event_id, event_name, event_key, kafka_topic, kafka_partition, kafka_offset, payload)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

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
)

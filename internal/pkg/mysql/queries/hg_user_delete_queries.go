package SQLQueriesPackage

// HGUserDeleteCanonicalIDSQL 按唯一键回读真实 user_id，避免 MySQL 大小写不敏感匹配与 Redis key 不一致。
const HGUserDeleteCanonicalIDSQL = "SELECT user_id FROM users WHERE user_id = ? LIMIT 1"

// HGUserDeleteLockSQL 锁定 users.user_id 唯一索引对应的账号行；删除事务内先锁定目标。
const HGUserDeleteLockSQL = "SELECT id FROM users WHERE user_id = ? FOR UPDATE"

// HGUserDeleteWalletSQL 锁定 000036 迁移后的 account 权威钱包；可用/冻结余额均须为零。
const HGUserDeleteWalletSQL = "SELECT balance, frozen_balance FROM account WHERE user_id = ? FOR UPDATE"

// HGUserDeleteWalletRowSQL 事务内删除余额为零且已通过全量关联校验的初始化钱包。
const HGUserDeleteWalletRowSQL = "DELETE FROM account WHERE user_id = ? AND balance = 0 AND frozen_balance = 0"

// HGUserDeleteAccountSQL 删除已通过全部检查的 users 行；user_security 的 CASCADE 仅清理认证副本。
const HGUserDeleteAccountSQL = "DELETE FROM users WHERE user_id = ?"

// HGUserDeleteAuditSQL 与账号删除同事务记录执行人及目标，不保留手机号等敏感数据。
const HGUserDeleteAuditSQL = "INSERT INTO user_directory_delete_audit (operator_user_id, target_user_id) VALUES (?, ?)"

// HGUserDeleteDependencies 是待删除账号的权威关联记录点查，任一命中即拒绝物理删除。
// 每条查询使用 user_id 等值条件和 LIMIT 1，必须具备对应字段前缀索引；没有索引时先部署配套迁移。
// 新增用户归属表时必须同步扩展此清单及索引，查询失败一律拒绝删除。
var HGUserDeleteDependencies = []string{
	"SELECT 1 FROM user_addresses WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM video_submissions WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM video_files WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM video_scheduled_publish WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM video_commercial_promotion WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM video_user_interactions WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM video_share_records WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM user_follow_relations WHERE follower_id = ? LIMIT 1",
	"SELECT 1 FROM user_follow_relations WHERE followee_id = ? LIMIT 1",
	"SELECT 1 FROM user_follow_stat_shards WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM user_coin_commands WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM user_coin_ledger WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM coin_asset_requests WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM account_ledger WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM coin_asset_lots WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM ops_coin_correction WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM ops_coin_correction WHERE applicant_id = ? LIMIT 1",
	"SELECT 1 FROM ops_coin_correction WHERE approver_id = ? LIMIT 1",
	"SELECT 1 FROM ops_asset_audit WHERE target_user_id = ? LIMIT 1",
	"SELECT 1 FROM ops_asset_audit WHERE operator_id = ? LIMIT 1",
	"SELECT 1 FROM ops_asset_audit WHERE applicant_id = ? LIMIT 1",
	"SELECT 1 FROM ops_asset_audit WHERE approver_id = ? LIMIT 1",
	"SELECT 1 FROM video_comments WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM video_comments WHERE reply_to_user_id = ? LIMIT 1",
	"SELECT 1 FROM video_comment_reactions WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM video_comment_image_quotas WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM video_comment_images WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM video_danmaku WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM wallet_recharge_orders WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM trade_order WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM trade_order_item WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM payment_recharge_order WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM payment_order WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM refund_order WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM mall_merchant WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM mall_fulfillment WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM mall_pending_order WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM mall_product_image WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM mall_product_edit WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM iam_user_role_bindings WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM iam_mfa_factors WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM iam_approval_requests WHERE applicant_id = ? LIMIT 1",
	"SELECT 1 FROM iam_approval_requests WHERE approver_id = ? LIMIT 1",
	"SELECT 1 FROM iam_audit_logs WHERE actor_user_id = ? LIMIT 1",
	"SELECT 1 FROM crawler_spider_switch_events WHERE actor_id = ? LIMIT 1",
	"SELECT 1 FROM user_directory_security_audit WHERE operator_user_id = ? LIMIT 1",
	"SELECT 1 FROM user_directory_security_audit WHERE target_user_id = ? LIMIT 1",
	"SELECT 1 FROM user_directory_delete_audit WHERE operator_user_id = ? LIMIT 1",
	"SELECT 1 FROM admin_user WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM user WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM app_user WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM user_course_goods WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM orders WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM notification_inbox WHERE receiver_id = ? LIMIT 1",
	"SELECT 1 FROM notification_preferences WHERE receiver_id = ? LIMIT 1",
	"SELECT 1 FROM notification_unread_shard WHERE receiver_id = ? LIMIT 1",
	"SELECT 1 FROM notification_event_ledger WHERE receiver_id = ? LIMIT 1",
	"SELECT 1 FROM notification_inbox_archive WHERE receiver_id = ? LIMIT 1",
	"SELECT 1 FROM im_conversation WHERE user_low = ? LIMIT 1",
	"SELECT 1 FROM im_conversation WHERE user_high = ? LIMIT 1",
	"SELECT 1 FROM im_conversation_member WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM im_message WHERE sender_id = ? LIMIT 1",
	"SELECT 1 FROM im_device WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM im_device_delivery WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM im_media WHERE owner_id = ? LIMIT 1",
	"SELECT 1 FROM im_realtime_job WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM im_private_attachment WHERE owner_id = ? LIMIT 1",
	"SELECT 1 FROM im_call_busy WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM im_call_session WHERE caller_id = ? LIMIT 1",
	"SELECT 1 FROM im_call_session WHERE callee_id = ? LIMIT 1",
	"SELECT 1 FROM im_friend_request WHERE requester_id = ? LIMIT 1",
	"SELECT 1 FROM im_friend_request WHERE addressee_id = ? LIMIT 1",
	"SELECT 1 FROM im_friendship WHERE user_low_id = ? LIMIT 1",
	"SELECT 1 FROM im_friendship WHERE user_high_id = ? LIMIT 1",
	"SELECT 1 FROM im_friend_remark WHERE owner_user_id = ? LIMIT 1",
	"SELECT 1 FROM im_friend_remark WHERE friend_user_id = ? LIMIT 1",
	"SELECT 1 FROM im_user_block WHERE blocker_user_id = ? LIMIT 1",
	"SELECT 1 FROM im_user_block WHERE blocked_user_id = ? LIMIT 1",
}

// HGUserDeleteNumericDependencies 防御旧课程/资源表的 BIGINT 用户主键引用。
// 部分旧表的 user_id 口径未统一，按 users.id 保守拒绝同值账号，不依赖字符串隐式转换。
var HGUserDeleteNumericDependencies = []string{
	"SELECT 1 FROM wechat_user WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM order_items WHERE user_id = ? LIMIT 1",
	"SELECT 1 FROM resource_upload_files WHERE user_id = ? LIMIT 1",
}

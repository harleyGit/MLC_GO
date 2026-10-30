package SQLQueriesPackage

// 以下 SQL 仅以会话或用户 ID 点查/有界游标读取；会话行锁负责单聊 seq 顺序，热点单聊须限流或水平分片。
const (
	HGIMUserExistsSQL         = `SELECT 1 FROM users WHERE user_id=?`
	HGIMCreateConversationSQL = `INSERT INTO im_conversation(user_low,user_high) VALUES (?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`
	HGIMAddMemberSQL          = `INSERT INTO im_conversation_member(conversation_id,user_id) VALUES (?,?) ON DUPLICATE KEY UPDATE user_id=user_id`
	HGIMConversationMemberSQL = `SELECT 1 FROM im_conversation_member WHERE conversation_id=? AND user_id=?`
	HGIMLockConversationSQL   = `SELECT user_low,user_high,last_seq FROM im_conversation WHERE id=? FOR UPDATE`
	HGIMExistingMessageSQL    = `SELECT id,conversation_id,seq,message_type,content_hash FROM im_message WHERE sender_id=? AND client_msg_id=?`
	HGIMAdvanceSeqSQL         = `UPDATE im_conversation SET last_seq=? WHERE id=? AND last_seq=?`
	HGIMInsertMessageSQL      = `INSERT INTO im_message(conversation_id,seq,sender_id,client_msg_id,message_type,content,content_hash) VALUES (?,?,?,?,?,?,?)`
	HGIMHistorySQL            = `SELECT id,seq,sender_id,message_type,content,created_at FROM im_message WHERE conversation_id=? AND seq>? ORDER BY seq ASC LIMIT ?`
	HGIMConversationPageSQL   = `SELECT c.id,c.last_seq,m.last_read_seq,c.updated_at FROM im_conversation_member m JOIN im_conversation c ON c.id=m.conversation_id WHERE m.user_id=? AND c.id<? ORDER BY c.id DESC LIMIT ?`
	HGIMReadSQL               = `UPDATE im_conversation_member SET last_read_seq=GREATEST(last_read_seq,?) WHERE conversation_id=? AND user_id=?`
	HGIMActiveDevicesSQL      = `SELECT id FROM im_device WHERE user_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY ORDER BY id LIMIT 6`
	HGIMInsertDeliverySQL     = `INSERT INTO im_device_delivery(device_id,message_id,user_id) VALUES (?,?,?)`
	HGIMLockDeviceOwnerSQL    = `SELECT 1 FROM users WHERE user_id=? FOR UPDATE`
	HGIMCreateDeviceSQL       = `INSERT INTO im_device(id,user_id,credential_hash) VALUES (?,?,?)`
	HGIMDeviceCredentialSQL   = `SELECT credential_hash FROM im_device WHERE id=? AND user_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY`
	HGIMTouchDeviceSQL        = `UPDATE im_device SET last_active_at=NOW(3) WHERE id=? AND user_id=? AND revoked_at IS NULL`
	HGIMRevokeDeviceSQL       = `UPDATE im_device SET revoked_at=NOW(3) WHERE id=? AND user_id=? AND revoked_at IS NULL`
	HGIMPendingDeviceSQL      = `SELECT d.message_id,m.conversation_id,m.seq FROM im_device_delivery d JOIN im_message m ON m.id=d.message_id WHERE d.device_id=? AND d.user_id=? AND d.delivered_at IS NULL AND d.message_id>? ORDER BY d.message_id ASC LIMIT ?`
	HGIMAcknowledgeDeviceSQL  = `UPDATE im_device_delivery SET delivered_at=NOW(3),lease_token=NULL,lease_until=NULL WHERE device_id=? AND user_id=? AND message_id=? AND delivered_at IS NULL`
)

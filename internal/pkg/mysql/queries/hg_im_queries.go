package SQLQueriesPackage

// 以下 SQL 仅以会话或用户 ID 点查/有界游标读取；会话行锁负责单聊 seq 顺序，热点单聊须限流或水平分片。
const (
	HGIMUserExistsSQL         = `SELECT 1 FROM users WHERE user_id=?`
	HGIMCreateConversationSQL = `INSERT INTO im_conversation(user_low,user_high,created_at,updated_at) VALUES (?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`
	HGIMAddMemberSQL          = `INSERT INTO im_conversation_member(conversation_id,user_id) VALUES (?,?) ON DUPLICATE KEY UPDATE user_id=user_id`
	HGIMConversationMemberSQL = `SELECT 1 FROM im_conversation_member WHERE conversation_id=? AND user_id=?`
	HGIMLockConversationSQL   = `SELECT user_low,user_high,last_seq FROM im_conversation WHERE id=? FOR UPDATE`
	HGIMExistingMessageSQL    = `SELECT id,conversation_id,seq,message_type,content_hash FROM im_message WHERE sender_id=? AND client_msg_id=?`
	HGIMAdvanceSeqSQL         = `UPDATE im_conversation SET last_seq=?,updated_at=UTC_TIMESTAMP(3) WHERE id=? AND last_seq=?`
	// 消息时间由 MySQL 权威 UTC 毫秒时钟生成，不依赖连接 session 时区；会话行锁分配的 seq 保证同毫秒排序。
	// 点写消息并通过 uk_im_conversation_seq / uk_im_sender_client_id 约束顺序及幂等；高热点会话仍受单行锁限制。
	HGIMInsertMessageSQL = `INSERT INTO im_message(conversation_id,seq,sender_id,client_msg_id,message_type,content,content_hash,created_at) VALUES (?,?,?,?,?,?,?,UTC_TIMESTAMP(3))`
	HGIMHistorySQL       = `SELECT id,seq,sender_id,message_type,content,created_at FROM im_message WHERE conversation_id=? AND seq>? ORDER BY seq ASC LIMIT ?`
	// 按 uk_im_conversation_seq 读取单会话最早仍存在的 seq；物理删历史后只允许客户端跳过此前缀。
	HGIMHistoryFloorSQL            = `SELECT seq FROM im_message WHERE conversation_id=? ORDER BY seq ASC LIMIT 1`
	HGIMConversationPageSQL        = `SELECT c.id,c.last_seq,m.last_read_seq,c.updated_at,c.user_low,c.user_high,m.muted FROM im_conversation_member m JOIN im_conversation c ON c.id=m.conversation_id WHERE m.user_id=? AND c.id<? AND (m.deleted_at IS NULL OR c.last_seq>m.deleted_seq) ORDER BY c.id DESC LIMIT ?`
	HGIMConversationByIDSQL        = `SELECT c.last_seq,m.last_read_seq,c.updated_at,c.user_low,c.user_high,m.muted FROM im_conversation_member m JOIN im_conversation c ON c.id=m.conversation_id WHERE m.conversation_id=? AND m.user_id=? LIMIT 1`
	HGIMRestoreConversationSQL     = `UPDATE im_conversation_member SET deleted_at=NULL,deleted_seq=0 WHERE conversation_id=? AND user_id=?`
	HGIMDeleteProgressSQL          = `SELECT deleted_seq,deleted_at FROM im_conversation_member WHERE conversation_id=? AND user_id=?`
	HGIMStageConversationDeleteSQL = `UPDATE im_conversation_member SET deleted_at=NULL,deleted_seq=? WHERE conversation_id=? AND user_id=?`
	HGIMSetMutedSQL                = `UPDATE im_conversation_member SET muted=? WHERE conversation_id=? AND user_id=?`
	// 按会话+created_at 索引有界查询；固定 UTC 截止时间，避免深 OFFSET。删除前需锁定会话行以排斥同时发送。
	HGIMOldMessagesSQL = `SELECT id FROM im_message FORCE INDEX(idx_im_message_retention) WHERE conversation_id=? AND created_at<? ORDER BY created_at,id LIMIT 21`
	// 关联记录须先于消息删除；idx_im_job_message、idx_im_delivery_message 与附件映射主键点写。
	// 每事务最多 20 条消息，失败可重试；对象存储媒体需另行授权清理，不在事务内访问外部服务。
	HGIMDeleteJobSQL            = `DELETE FROM im_realtime_job WHERE message_id=?`
	HGIMDeleteDeliverySQL       = `DELETE FROM im_device_delivery WHERE message_id=?`
	HGIMDeleteAttachmentLinkSQL = `DELETE FROM im_message_attachment WHERE message_id=?`
	HGIMDeleteMessageSQL        = `DELETE FROM im_message WHERE id=? AND conversation_id=?`
	HGIMHideConversationSQL     = `UPDATE im_conversation_member SET deleted_at=UTC_TIMESTAMP(3),deleted_seq=? WHERE conversation_id=? AND user_id=?`
	HGIMReadSQL                 = `UPDATE im_conversation_member SET last_read_seq=GREATEST(last_read_seq,?) WHERE conversation_id=? AND user_id=?`
	HGIMActiveDevicesSQL        = `SELECT id FROM im_device WHERE user_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY ORDER BY id LIMIT 6`
	// 最早登录设备：按首次注册时间排序，同毫秒以设备 ID 稳定打破并列；走 idx_im_device_oldest，旧失活行需定期归档。
	HGIMOldestDeviceSQL = `SELECT id,session_id FROM im_device WHERE user_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY ORDER BY created_at,id LIMIT 1`
	// 所有新会话鉴权均按 (user_id,session_id,forced_logout_at) 点查；不能用只在客户端清理 token 替代服务端撤销。
	HGIMForcedLogoutSQL        = `SELECT 1 FROM im_device WHERE user_id=? AND session_id=? AND forced_logout_at IS NOT NULL LIMIT 1`
	HGIMSessionActiveDeviceSQL = `SELECT 1 FROM im_device WHERE user_id=? AND session_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY LIMIT 1`
	// 同登录会话凭据遗失时只选一台本人的活跃设备；索引 idx_im_device_session_logout 定位，同用户活跃设备上限五台。
	HGIMOwnOldestDeviceSQL     = `SELECT id FROM im_device WHERE user_id=? AND session_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY ORDER BY created_at,id LIMIT 1`
	HGIMInsertDeliverySQL      = `INSERT INTO im_device_delivery(device_id,message_id,user_id) VALUES (?,?,?)`
	HGIMLockDeviceOwnerSQL     = `SELECT 1 FROM users WHERE user_id=? FOR UPDATE`
	HGIMCreateDeviceSQL        = `INSERT INTO im_device(id,user_id,credential_hash) VALUES (?,?,?)`
	HGIMCreateSessionDeviceSQL = `INSERT INTO im_device(id,user_id,credential_hash,session_id) VALUES (?,?,?,?)`
	HGIMDeviceCredentialSQL    = `SELECT credential_hash FROM im_device WHERE id=? AND user_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY`
	HGIMTouchDeviceSQL         = `UPDATE im_device SET last_active_at=NOW(3) WHERE id=? AND user_id=? AND revoked_at IS NULL`
	HGIMRevokeDeviceSQL        = `UPDATE im_device SET revoked_at=NOW(3) WHERE id=? AND user_id=? AND revoked_at IS NULL`
	// 旧版设备无可关联的登录会话，仅撤销最早那台 IM 设备；主键点写且再次核实旧记录，不误删其他用户设备。
	HGIMRevokeLegacyDeviceSQL = `UPDATE im_device SET revoked_at=NOW(3) WHERE id=? AND user_id=? AND session_id IS NULL AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY`
	// 同会话设备替换仅点写所选设备；不设置 forced_logout_at，避免把当前登录会话踢下线。
	HGIMRevokeOwnDeviceSQL = `UPDATE im_device SET revoked_at=NOW(3) WHERE id=? AND user_id=? AND session_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY`
	// 同会话最多替换五台活跃设备；按 idx_im_device_session_logout 定位，ORDER BY/LIMIT 防止异常存量大范围更新。
	HGIMForceLogoutDeviceSQL = `UPDATE im_device SET revoked_at=NOW(3),forced_logout_at=NOW(3) WHERE user_id=? AND session_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY ORDER BY id LIMIT 5`
	HGIMPendingDeviceSQL     = `SELECT d.message_id,m.conversation_id,m.seq FROM im_device_delivery d JOIN im_message m ON m.id=d.message_id WHERE d.device_id=? AND d.user_id=? AND d.delivered_at IS NULL AND d.message_id>? ORDER BY d.message_id ASC LIMIT ?`
	HGIMAcknowledgeDeviceSQL = `UPDATE im_device_delivery SET delivered_at=NOW(3),lease_token=NULL,lease_until=NULL WHERE device_id=? AND user_id=? AND message_id=? AND delivered_at IS NULL`
)

package SQLQueriesPackage

// 呼叫均通过随机主键或唯一幂等键点查；两个忙线占位按 user_low/user_high 固定顺序写入和释放。
const (
	HGIMCallInsertSQL      = `INSERT INTO im_call_session(id,conversation_id,caller_id,callee_id,request_id,caller_device,media_type,expires_at) VALUES(?,?,?,?,?,?,?,NOW(3)+INTERVAL 45 SECOND)`
	HGIMCallColumns        = `id,conversation_id,caller_id,callee_id,caller_device,callee_device,media_type,state,version,CAST(UNIX_TIMESTAMP(expires_at)*1000 AS UNSIGNED)`
	HGIMCallGetSQL         = `SELECT ` + HGIMCallColumns + ` FROM im_call_session WHERE id=?`
	HGIMCallLockSQL        = HGIMCallGetSQL + ` FOR UPDATE`
	HGIMCallRequestSQL     = `SELECT ` + HGIMCallColumns + ` FROM im_call_session WHERE caller_id=? AND request_id=?`
	HGIMCallBusySQL        = `INSERT INTO im_call_busy(user_id,call_id) VALUES(?,?)`
	HGIMCallFreeSQL        = `DELETE FROM im_call_busy WHERE user_id=? AND call_id=?`
	HGIMCallAcceptSQL      = `UPDATE im_call_session SET state='accepted',callee_device=?,version=version+1,expires_at=NOW(3)+INTERVAL 1 HOUR WHERE id=? AND state='ringing' AND expires_at>NOW(3)`
	HGIMCallEndSQL         = `UPDATE im_call_session SET state=?,version=version+1,cleanup_at=NOW(3),cleanup_until=GREATEST(COALESCE(cleanup_until,NOW(3)),NOW(3)+INTERVAL 120 SECOND) WHERE id=? AND state IN ('ringing','accepted')`
	HGIMCallTokenWindowSQL = `UPDATE im_call_session SET cleanup_until=GREATEST(COALESCE(cleanup_until,NOW(3)),NOW(3)+INTERVAL 120 SECOND) WHERE id=? AND state='accepted' AND expires_at>NOW(3)`
	HGIMCallEventSQL       = `SELECT conversation_id,caller_id,callee_id,version FROM im_call_session WHERE id=?`
	// idx_im_call_expiry 每轮按状态仅取一条；终态不参与扫描，不全表统计。
	HGIMCallExpiredSQL       = `SELECT id FROM im_call_session WHERE state=? AND expires_at<=NOW(3) ORDER BY expires_at,id LIMIT 1`
	HGIMCallCleanupSQL       = `SELECT id FROM im_call_session WHERE cleanup_at<=NOW(3) ORDER BY cleanup_at,id LIMIT 4`
	HGIMCallCleanedSQL       = `UPDATE im_call_session SET cleanup_at=IF(cleanup_until>NOW(3),NOW(3)+INTERVAL 5 SECOND,NULL) WHERE id=? AND state NOT IN ('ringing','accepted')`
	HGIMCallCleanupRetrySQL  = `UPDATE im_call_session SET cleanup_at=NOW(3)+INTERVAL 10 SECOND WHERE id=? AND cleanup_at IS NOT NULL`
	HGIMCallRoomSIDSQL       = `SELECT room_sid FROM im_call_session WHERE id=?`
	HGIMCallExpiredFlagSQL  = `SELECT expires_at<=NOW(3) FROM im_call_session WHERE id=?`
	HGIMCallSetRoomSIDSQL    = `UPDATE im_call_session SET room_sid=? WHERE id=? AND state='accepted'`
	HGIMCallWebhookInsertSQL = `INSERT INTO im_call_webhook(event_id,payload_hash) VALUES(?,?) ON DUPLICATE KEY UPDATE event_id=event_id`
	HGIMCallWebhookHashSQL   = `SELECT payload_hash FROM im_call_webhook WHERE event_id=? FOR UPDATE`
)

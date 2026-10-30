package SQLQueriesPackage

// IM 实时查询依赖 000054；Kafka 消费只持久化有界设备任务，事务内没有网络请求。
const (
	HGIMInboxInsertSQL  = `INSERT INTO im_realtime_inbox(event_id,payload_hash,topic,partition_id,offset_id) VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE event_id=event_id`
	HGIMInboxHashSQL    = `SELECT payload_hash FROM im_realtime_inbox WHERE event_id=? FOR UPDATE`
	HGIMEventMessageSQL = `SELECT conversation_id,seq FROM im_message WHERE id=?`
	HGIMEventDevicesSQL = `SELECT d.device_id,d.user_id FROM im_device_delivery d JOIN im_device v ON v.id=d.device_id AND v.user_id=d.user_id WHERE d.message_id=? AND d.delivered_at IS NULL AND v.revoked_at IS NULL AND v.last_active_at>NOW(3)-INTERVAL 30 DAY ORDER BY d.device_id LIMIT 11`
	HGIMJobInsertSQL    = `INSERT INTO im_realtime_job(event_id,device_id,user_id,message_id,payload) VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE id=id`
	// 单状态范围走 idx_im_job_due；租约到期可接管，单次最多四条，避免持锁外部 I/O。
	HGIMJobClaimSQL          = `SELECT id,device_id,user_id,message_id,payload,attempts FROM im_realtime_job WHERE state=? AND next_retry_at<=NOW(3) ORDER BY next_retry_at,id LIMIT 4 FOR UPDATE SKIP LOCKED`
	HGIMJobLeaseSQL          = `UPDATE im_realtime_job SET state=1,lease_token=?,next_retry_at=NOW(3)+INTERVAL 30 SECOND,attempts=LEAST(attempts+1,12) WHERE id=?`
	HGIMJobFinishSQL         = `UPDATE im_realtime_job SET state=?,lease_token=NULL,next_retry_at=TIMESTAMPADD(SECOND,?,NOW(3)) WHERE id=? AND state=1 AND lease_token=?`
	HGIMJobDeviceActiveSQL   = `SELECT 1 FROM im_device WHERE id=? AND user_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY`
	HGIMJobMessagePendingSQL = `SELECT 1 FROM im_device_delivery WHERE device_id=? AND message_id=? AND user_id=? AND delivered_at IS NULL`
)

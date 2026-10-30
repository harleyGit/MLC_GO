package SQLQueriesPackage

// 私有附件通过随机主键/owner 索引访问；上传配额最多扫描 101 条，锁 users 主键串行预留。
const (
	HGIMMediaQuotaSQL    = `SELECT byte_size FROM im_private_attachment WHERE owner_id=? ORDER BY created_at,id LIMIT 101`
	HGIMMediaInsertSQL   = `INSERT INTO im_private_attachment(id,owner_id,conversation_id,object_key,media_type,mime_type,byte_size,sha256,upload_expires_at) VALUES(?,?,?,?,?,?,?,?,NOW(3)+INTERVAL 10 MINUTE)`
	HGIMMediaGetSQL      = `SELECT owner_id,conversation_id,object_key,immutable_key,media_type,mime_type,byte_size,sha256,state,width,height,duration_ms FROM im_private_attachment WHERE id=?`
	HGIMMediaLockSQL     = HGIMMediaGetSQL + ` FOR UPDATE`
	HGIMMediaCompleteSQL = `UPDATE im_private_attachment SET state='verifying',next_check_at=NOW(3) WHERE id=? AND owner_id=? AND state='uploading' AND upload_expires_at>NOW(3)`
	HGIMMediaDeleteSQL   = `UPDATE im_private_attachment SET state='deleted',lease_token=NULL WHERE id=? AND owner_id=?`
	HGIMMediaLinkSQL     = `INSERT INTO im_message_attachment(message_id,attachment_id,conversation_id) VALUES(?,?,?)`
	HGIMMediaReadableSQL = `SELECT 1 FROM im_message_attachment WHERE attachment_id=? AND conversation_id=? LIMIT 1`
	// idx_im_attachment_review 限定审核任务；外部审核在提交租约后进行。
	HGIMMediaClaimSQL    = `SELECT id,object_key,mime_type,byte_size,sha256,media_type,attempts FROM im_private_attachment WHERE state='verifying' AND next_check_at<=NOW(3) ORDER BY next_check_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`
	HGIMMediaLeaseSQL    = `UPDATE im_private_attachment SET lease_token=?,attempts=LEAST(attempts+1,12),next_check_at=NOW(3)+INTERVAL 30 SECOND WHERE id=? AND state='verifying'`
	HGIMMediaReviewedSQL = `UPDATE im_private_attachment SET state=?,immutable_key=?,width=?,height=?,duration_ms=?,lease_token=NULL,next_check_at=NOW(3)+INTERVAL 30 SECOND WHERE id=? AND state='verifying' AND lease_token=?`
)

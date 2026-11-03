package SQLQueriesPackage

// 联系人列表按内部用户主键排序并限制 100 条；friendship 两端分别走主键及 idx_im_friend_high。
// 显示 ID 使用 users.id 推导的稳定公开编号，不能把 JWT user_id 或私有手机号当公开编号。
// users.user_id 为 utf8mb4_general_ci；新联系人表可能继承库默认的 utf8mb4_0900_ai_ci。
// JOIN 在被查找列的另一侧指定排序规则；备注用 utf8mb4 默认规则（与新表一致）转换用户 ID，
// 避免 1267 且保留 users.user_id / 备注复合索引查找。
const (
	HGIMContactFriendsSQL        = `SELECT u.user_id,CONCAT('U',LPAD(u.id,20,'0')),COALESCE(u.user_name,''),COALESCE(u.nickname,''),COALESCE(u.signature,''),COALESCE(r.remark,''),COALESCE(CAST(r.tags AS CHAR),'[]'),COALESCE(r.phone,''),COALESCE(r.description,'') FROM im_friendship f JOIN users u ON u.user_id=IF(f.user_low_id=?,f.user_high_id,f.user_low_id) COLLATE utf8mb4_general_ci LEFT JOIN im_friend_remark r ON r.owner_user_id=? AND r.friend_user_id=CAST(u.user_id AS CHAR CHARACTER SET utf8mb4) WHERE (f.user_low_id=? OR f.user_high_id=?) AND u.id>? ORDER BY u.id LIMIT ?`
	HGIMContactBlocksSQL         = `SELECT u.user_id,CONCAT('U',LPAD(u.id,20,'0')),COALESCE(u.user_name,''),COALESCE(u.nickname,''),b.block_note,b.created_at FROM im_user_block b JOIN users u ON u.user_id=b.blocked_user_id COLLATE utf8mb4_general_ci WHERE b.blocker_user_id=? AND u.id>? ORDER BY u.id LIMIT ?`
	HGIMContactRequestsSQL       = `SELECT q.request_id,u.user_id,CONCAT('U',LPAD(u.id,20,'0')),COALESCE(u.user_name,''),COALESCE(u.nickname,''),q.message,q.created_at FROM im_friend_request q JOIN users u ON u.user_id=q.requester_id COLLATE utf8mb4_general_ci WHERE q.addressee_id=? AND q.status=0 AND q.id>? ORDER BY q.id LIMIT ?`
	HGIMContactProfileSQL        = `SELECT u.user_id,CONCAT('U',LPAD(u.id,20,'0')),COALESCE(u.user_name,''),COALESCE(u.nickname,''),COALESCE(u.signature,'') FROM users u WHERE u.user_id=? LIMIT 1`
	HGIMContactSearchPublicSQL   = `SELECT u.user_id,CONCAT('U',LPAD(u.id,20,'0')),COALESCE(u.user_name,''),COALESCE(u.nickname,''),COALESCE(u.signature,'') FROM users u WHERE u.id=? LIMIT 1`
	HGIMContactSearchUsernameSQL = `SELECT u.user_id,CONCAT('U',LPAD(u.id,20,'0')),COALESCE(u.user_name,''),COALESCE(u.nickname,''),COALESCE(u.signature,'') FROM users u WHERE u.user_name=? LIMIT 1`
	HGIMContactLockUserSQL       = `SELECT 1 FROM users WHERE user_id=? FOR UPDATE`
	HGIMContactPairSQL           = `SELECT 1 FROM im_friendship WHERE user_low_id=? AND user_high_id=? FOR UPDATE`
	HGIMContactBlockPairSQL      = `SELECT 1 FROM im_user_block WHERE (blocker_user_id=? AND blocked_user_id=?) OR (blocker_user_id=? AND blocked_user_id=?) LIMIT 1`
	HGIMContactPendingSQL        = `SELECT request_id,requester_id,addressee_id FROM im_friend_request WHERE ((requester_id=? AND addressee_id=?) OR (requester_id=? AND addressee_id=?)) AND status=0 ORDER BY id DESC LIMIT 1 FOR UPDATE`
	HGIMContactInsertRequestSQL  = `INSERT INTO im_friend_request(request_id,requester_id,addressee_id,message) VALUES (?,?,?,?)`
	HGIMContactLockRequestSQL    = `SELECT requester_id,addressee_id,status FROM im_friend_request WHERE request_id=? FOR UPDATE`
	HGIMContactRequestSQL        = `SELECT requester_id,addressee_id,status FROM im_friend_request WHERE request_id=?`
	HGIMContactDecideRequestSQL  = `UPDATE im_friend_request SET status=? WHERE request_id=? AND status=0`
	HGIMContactAcceptPairSQL     = `INSERT INTO im_friendship(user_low_id,user_high_id) VALUES (?,?)`
	HGIMContactDeletePairSQL     = `DELETE FROM im_friendship WHERE user_low_id=? AND user_high_id=?`
	HGIMContactUpsertRemarkSQL   = `INSERT INTO im_friend_remark(owner_user_id,friend_user_id,remark,tags,phone,description) VALUES (?,?,?,?,?,?) ON DUPLICATE KEY UPDATE remark=VALUES(remark),tags=VALUES(tags),phone=VALUES(phone),description=VALUES(description)`
	HGIMContactBlockSQL          = `INSERT INTO im_user_block(blocker_user_id,blocked_user_id,block_note) VALUES (?,?,?) ON DUPLICATE KEY UPDATE block_note=VALUES(block_note)`
	HGIMContactRejectPendingSQL  = `UPDATE im_friend_request SET status=2 WHERE ((requester_id=? AND addressee_id=?) OR (requester_id=? AND addressee_id=?)) AND status=0 ORDER BY id LIMIT 100`
	HGIMContactUnblockSQL        = `DELETE FROM im_user_block WHERE blocker_user_id=? AND blocked_user_id=?`
	HGIMContactStatusSQL         = `SELECT EXISTS(SELECT 1 FROM im_friendship WHERE user_low_id=? AND user_high_id=?), EXISTS(SELECT 1 FROM im_user_block WHERE (blocker_user_id=? AND blocked_user_id=?) OR (blocker_user_id=? AND blocked_user_id=?)), EXISTS(SELECT 1 FROM im_friend_request WHERE requester_id=? AND addressee_id=? AND status=0), EXISTS(SELECT 1 FROM im_friend_request WHERE requester_id=? AND addressee_id=? AND status=0)`
)

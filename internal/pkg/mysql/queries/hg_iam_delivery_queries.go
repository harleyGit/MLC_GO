package SQLQueriesPackage

// HGInsertIAMDeliveryJob 与 IAM 审计同一短事务写入；仅 user/role 目标，无法被客户端直接写入。
const HGInsertIAMDeliveryJob = `INSERT INTO iam_access_delivery_jobs(target_type,target_id,role_name) VALUES (?,?,?)`

// HGClaimIAMDeliveryJob 以状态、到期时间和主键索引限 1 条，不锁住执行中的外部网关 I/O。
const HGClaimIAMDeliveryJob = `SELECT id,target_type,target_id,role_name,last_user_id FROM iam_access_delivery_jobs WHERE status='pending' AND available_at<=CURRENT_TIMESTAMP(3) ORDER BY available_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`
const HGLeaseIAMDeliveryJob = `UPDATE iam_access_delivery_jobs SET lease_token=?,available_at=DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL 30 SECOND) WHERE id=? AND status='pending'`

// HGRoleDeliveryUsers 按角色 ID 和 user_id 索引游标扫描；含撤销绑定，以便角色停用时通知历史目标。
const HGRoleDeliveryUsers = `SELECT DISTINCT user_id FROM iam_user_role_bindings WHERE role_id=? AND user_id>? ORDER BY user_id LIMIT 51`
const HGRoleDeliveryName = `SELECT role_name FROM iam_roles WHERE id=?`
const HGUserDeliveryName = `SELECT role_name FROM iam_roles WHERE role_code=? ORDER BY id LIMIT 1`

// HGFinishIAMDeliveryJob 与租约令牌及前一游标比较，失效工作者不可写回旧游标。
const HGFinishIAMDeliveryJob = `UPDATE iam_access_delivery_jobs SET last_user_id=?,status=?,lease_token='',available_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status='pending' AND lease_token=? AND last_user_id=?`
const HGRetryIAMDeliveryJob = `UPDATE iam_access_delivery_jobs SET lease_token='',available_at=DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL 5 SECOND) WHERE id=? AND status='pending' AND lease_token=?`

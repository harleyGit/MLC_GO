package SQLQueriesPackage

// IAM 固定 SQL 集中于此。角色、绑定均按主键/唯一键点查；列表使用 id 游标和有界 LIMIT。
const (
	// 权限点查须使用 idx_iam_binding_user_scope、role_permissions 主键及 permission_code 唯一键；调用方失败时拒绝。
	HGSelectIAMPermission = `SELECT 1 FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id AND r.status=1 JOIN iam_role_permissions rp ON rp.role_id=r.id JOIN iam_permissions p ON p.id=rp.permission_id LEFT JOIN iam_binding_permission_limits lim ON lim.binding_id=b.id LEFT JOIN iam_binding_allowed_permissions allowp ON allowp.binding_id=b.id AND allowp.permission_id=p.id WHERE b.user_id=? AND b.status=1 AND b.scope_type=? AND b.scope_id=? AND (b.expires_at IS NULL OR b.expires_at>CURRENT_TIMESTAMP(3)) AND p.permission_code=? AND (lim.binding_id IS NULL OR allowp.binding_id IS NOT NULL) LIMIT 1`
	HGSelectIAMManager    = `SELECT 1 FROM admin_user WHERE user_id=? AND status=1 AND is_delete=0 LIMIT 1`
	// 即时授权可作用于现有普通账号；users.user_id 唯一键点查，不凭手机号或客户端声称的身份授予。
	HGSelectIAMGrantTargetUser = `SELECT 1 FROM users WHERE user_id=? LIMIT 1`
	// 自授判断必须使用 users.user_id 的实际数据库排序规则；同一账号仅大小写不同也不能通过。
	HGSelectIAMGrantSelf      = `SELECT 1 FROM users WHERE user_id=? AND user_id=? LIMIT 1`
	HGSelectIAMPolicyRevision = `SELECT revision FROM iam_policy_revision WHERE id=1`
	// 当前用户有效授权投影：idx_iam_binding_user_scope 按 user_id 定位绑定，角色/权限表通过主键联查。
	// 最多读取 1001 行，调用方超过 1000 行须拒绝而非截断授权；不使用 OFFSET。
	HGSelectIAMAccessGrants = `SELECT r.role_code,p.permission_code,b.scope_type,b.scope_id,b.expires_at FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id AND r.status=1 JOIN iam_role_permissions rp ON rp.role_id=r.id JOIN iam_permissions p ON p.id=rp.permission_id LEFT JOIN iam_binding_permission_limits lim ON lim.binding_id=b.id LEFT JOIN iam_binding_allowed_permissions allowp ON allowp.binding_id=b.id AND allowp.permission_id=p.id WHERE b.user_id=? AND b.status=1 AND (b.expires_at IS NULL OR b.expires_at>CURRENT_TIMESTAMP(3)) AND (lim.binding_id IS NULL OR allowp.binding_id IS NOT NULL) ORDER BY b.id,p.permission_code LIMIT 1001`
	HGBumpIAMPolicyRevision = `UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1`
	// 缓存只保存永不过期绑定的放行结果；有 expires_at 的临时授权仍查询权威库。
	HGSelectIAMStablePermission = `SELECT 1 FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id AND r.status=1 JOIN iam_role_permissions rp ON rp.role_id=r.id JOIN iam_permissions p ON p.id=rp.permission_id LEFT JOIN iam_binding_permission_limits lim ON lim.binding_id=b.id LEFT JOIN iam_binding_allowed_permissions allowp ON allowp.binding_id=b.id AND allowp.permission_id=p.id WHERE b.user_id=? AND b.status=1 AND b.scope_type=? AND b.scope_id=? AND b.expires_at IS NULL AND p.permission_code=? AND (lim.binding_id IS NULL OR allowp.binding_id IS NOT NULL) LIMIT 1`
	// 管理端资产菜单仅需活动全局权限；依赖 user_scope 索引，LIMIT 防止权限目录异常膨胀。
	HGSelectIAMAssetPermissions  = `SELECT DISTINCT p.permission_code FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id AND r.status=1 JOIN iam_role_permissions rp ON rp.role_id=r.id JOIN iam_permissions p ON p.id=rp.permission_id LEFT JOIN iam_binding_permission_limits lim ON lim.binding_id=b.id LEFT JOIN iam_binding_allowed_permissions allowp ON allowp.binding_id=b.id AND allowp.permission_id=p.id WHERE b.user_id=? AND b.status=1 AND b.scope_type='global' AND b.scope_id='' AND (b.expires_at IS NULL OR b.expires_at>CURRENT_TIMESTAMP(3)) AND (lim.binding_id IS NULL OR allowp.binding_id IS NOT NULL) AND (p.permission_code LIKE 'asset:%' OR p.permission_code LIKE 'payment:recharge_sku:%') ORDER BY p.permission_code LIMIT 34`
	HGSelectIAMPassword          = `SELECT password_hash,salt FROM users WHERE user_id=? LIMIT 1`
	HGSelectIAMMFAFactor         = `SELECT secret_cipher,enabled,last_counter FROM iam_mfa_factors WHERE user_id=? FOR UPDATE`
	HGInsertIAMMFAFactor         = `INSERT INTO iam_mfa_factors(user_id,secret_cipher,enabled,last_counter) VALUES(?,?,0,-1) ON DUPLICATE KEY UPDATE secret_cipher=IF(enabled=0,VALUES(secret_cipher),secret_cipher),updated_at=CURRENT_TIMESTAMP(3)`
	HGActivateIAMMFAFactor       = `UPDATE iam_mfa_factors SET enabled=1,last_counter=?,updated_at=CURRENT_TIMESTAMP(3) WHERE user_id=? AND enabled=0 AND last_counter<?`
	HGConsumeIAMMFACounter       = `UPDATE iam_mfa_factors SET last_counter=?,updated_at=CURRENT_TIMESTAMP(3) WHERE user_id=? AND enabled=1 AND last_counter<?`
	HGInsertIAMApproval          = `INSERT INTO iam_approval_requests(request_id,applicant_id,action,payload,status,expires_at) VALUES(?,?,?,?,'pending',?)`
	HGSelectIAMApprovalForUpdate = `SELECT applicant_id,action,payload,status,expires_at FROM iam_approval_requests WHERE id=? LIMIT 1 FOR UPDATE`
	HGDecideIAMApproval          = `UPDATE iam_approval_requests SET status=?,approver_id=?,decided_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status='pending' AND expires_at>CURRENT_TIMESTAMP(3)`
	// 审批列表依赖 status,id 索引，仅返回待处理申请，游标递减且单页有上限。
	HGSelectIAMApprovalsFirst = `SELECT id,request_id,applicant_id,action,payload,expires_at,created_at FROM iam_approval_requests WHERE status='pending' AND expires_at>CURRENT_TIMESTAMP(3) ORDER BY id DESC LIMIT ?`
	HGSelectIAMApprovalsAfter = `SELECT id,request_id,applicant_id,action,payload,expires_at,created_at FROM iam_approval_requests WHERE status='pending' AND expires_at>CURRENT_TIMESTAMP(3) AND id<? ORDER BY id DESC LIMIT ?`
	// 目标管理员待审批授权查询依赖 idx_iam_approval_target_status_id：只读 pending 且未过期的 grant，id 倒序游标与 LIMIT 避免全表扫描。
	// target_user_id 为生成列，旧申请由 JSON 自动派生；仅在迁移 000074 应用后可用，未迁移时请求失败而不降级为无界过滤。
	HGSelectIAMPendingGrantsFirst = `SELECT id,payload,expires_at,created_at FROM iam_approval_requests WHERE target_user_id=? AND status='pending' AND expires_at>CURRENT_TIMESTAMP(3) ORDER BY id DESC LIMIT ?`
	HGSelectIAMPendingGrantsAfter = `SELECT id,payload,expires_at,created_at FROM iam_approval_requests WHERE target_user_id=? AND status='pending' AND expires_at>CURRENT_TIMESTAMP(3) AND id<? ORDER BY id DESC LIMIT ?`
	HGSelectIAMRoleVersion        = `SELECT version,built_in,role_code FROM iam_roles WHERE id=? AND status=1 LIMIT 1`
	// 创作者身份在首次上传前按 users 唯一业务 ID 建立；业务权限不进入 Redis 授权缓存。
	HGInsertIAMCreatorBinding = `INSERT IGNORE INTO iam_user_role_bindings(user_id,role_id,scope_type,scope_id,status,granted_by) SELECT u.user_id,r.id,'global','',1,'creator-onboarding' FROM users u JOIN iam_roles r ON r.role_code='creator' AND r.status=1 WHERE u.user_id=? LIMIT 1`
	// 商家审核通过后同事务赋予业务身份，撤销/拒绝申请均不自动扩权。
	HGInsertIAMMerchantBinding = `INSERT IGNORE INTO iam_user_role_bindings(user_id,role_id,scope_type,scope_id,status,granted_by) SELECT m.user_id,r.id,'global','',1,'merchant-onboarding' FROM mall_merchant m JOIN iam_roles r ON r.role_code='merchant' AND r.status=1 WHERE m.merchant_id=? AND m.user_id=? AND m.status=1 LIMIT 1`
	// 角色列表依赖 idx_iam_role_status_id，使用 id 游标，单页由 Service 限制到 100。
	HGSelectIAMRolesFirst             = `SELECT id,role_code,role_name,description,status,built_in,created_at FROM iam_roles WHERE status=1 ORDER BY id DESC LIMIT ?`
	HGSelectIAMRolesAfter             = `SELECT id,role_code,role_name,description,status,built_in,created_at FROM iam_roles WHERE status=1 AND id<? ORDER BY id DESC LIMIT ?`
	HGSelectIAMRole                   = `SELECT id,role_code,role_name,description,status,built_in,created_at FROM iam_roles WHERE id=? LIMIT 1`
	HGSelectIAMActiveRoleCode         = `SELECT role_code FROM iam_roles WHERE id=? AND status=1 LIMIT 1 FOR UPDATE`
	HGSelectIAMBindingRoleForRevoke   = `SELECT r.role_code FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id WHERE b.id=? AND b.user_id=? AND b.status=1 LIMIT 1 FOR UPDATE`
	HGSelectIAMRoleForPermissionWrite = `SELECT built_in,version,role_code FROM iam_roles WHERE id=? AND status=1 LIMIT 1 FOR UPDATE`
	HGInsertIAMRole                   = `INSERT INTO iam_roles(role_code,role_name,description,status,built_in) VALUES(?,?,?,1,0)`
	HGUpdateIAMRole                   = `UPDATE iam_roles SET role_name=?,description=?,version=version+1,updated_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status=1 AND built_in=0`
	HGDisableIAMRole                  = `UPDATE iam_roles SET status=0,updated_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status=1 AND built_in=0 AND NOT EXISTS(SELECT 1 FROM iam_user_role_bindings WHERE role_id=? AND status=1 AND (expires_at IS NULL OR expires_at>CURRENT_TIMESTAMP(3)) LIMIT 1)`
	// 员工绑定列表依赖 idx_iam_binding_user_page，精确 user_id 加 id 游标；已过期或停用角色不展示。
	HGSelectIAMBindingsFirst = `SELECT b.id,b.user_id,b.role_id,r.role_code,b.scope_type,b.scope_id,b.expires_at,EXISTS(SELECT 1 FROM iam_binding_permission_limits lim WHERE lim.binding_id=b.id) FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id AND r.status=1 WHERE b.user_id=? AND b.status=1 AND (b.expires_at IS NULL OR b.expires_at>CURRENT_TIMESTAMP(3)) ORDER BY b.id DESC LIMIT ?`
	HGSelectIAMBindingsAfter = `SELECT b.id,b.user_id,b.role_id,r.role_code,b.scope_type,b.scope_id,b.expires_at,EXISTS(SELECT 1 FROM iam_binding_permission_limits lim WHERE lim.binding_id=b.id) FROM iam_user_role_bindings b JOIN iam_roles r ON r.id=b.role_id AND r.status=1 WHERE b.user_id=? AND b.status=1 AND b.id<? AND (b.expires_at IS NULL OR b.expires_at>CURRENT_TIMESTAMP(3)) ORDER BY b.id DESC LIMIT ?`
	HGSelectIAMBinding       = `SELECT id,CASE WHEN expires_at IS NOT NULL AND expires_at<=CURRENT_TIMESTAMP(3) THEN 0 ELSE status END FROM iam_user_role_bindings WHERE user_id=? AND role_id=? AND scope_type=? AND scope_id=? FOR UPDATE`
	HGInsertIAMBinding       = `INSERT INTO iam_user_role_bindings(user_id,role_id,scope_type,scope_id,status,expires_at,granted_by) VALUES(?,?,?,?,1,?,?)`
	HGReactivateIAMBinding   = `UPDATE iam_user_role_bindings SET status=1,expires_at=?,granted_by=?,revoked_at=NULL,updated_at=CURRENT_TIMESTAMP(3) WHERE id=?`
	// 子集写入只用绑定唯一键或主键点查，role_permissions 通过角色主键有界读取；事务中角色行已被锁定。
	HGSelectIAMBindingIDForLimit = `SELECT id FROM iam_user_role_bindings WHERE user_id=? AND role_id=? AND scope_type=? AND scope_id=? AND status=1 LIMIT 1`
	HGInsertIAMBindingLimit      = `INSERT IGNORE INTO iam_binding_permission_limits(binding_id) VALUES(?)`
	// 撤销后重新授权需清除旧子集；外键级联删除允许权限行，避免旧接口意外继承历史限制。
	HGDeleteIAMBindingLimit   = `DELETE FROM iam_binding_permission_limits WHERE binding_id=?`
	HGDeleteIAMBindingAllowed = `DELETE FROM iam_binding_allowed_permissions WHERE binding_id=?`
	HGInsertIAMBindingAllowed = `INSERT INTO iam_binding_allowed_permissions(binding_id,permission_id) SELECT ?,id FROM iam_permissions WHERE permission_code=? LIMIT 1`
	HGRevokeIAMBinding        = `UPDATE iam_user_role_bindings SET status=0,revoked_at=CURRENT_TIMESTAMP(3),updated_at=CURRENT_TIMESTAMP(3) WHERE id=? AND user_id=? AND status=1`
	// 审计与授权变更同事务插入；审计表按 actor/target 时间索引查询，归档由独立任务负责。
	HGInsertIAMAudit           = `INSERT INTO iam_audit_logs(actor_user_id,action,target_type,target_id,request_id,detail) VALUES(?,?,?,?,?,?)`
	HGSelectIAMPermissions     = `SELECT permission_code,description FROM iam_permissions ORDER BY permission_code LIMIT ?`
	HGSelectIAMRolePermissions = `SELECT p.permission_code FROM iam_role_permissions rp JOIN iam_permissions p ON p.id=rp.permission_id WHERE rp.role_id=? ORDER BY p.permission_code LIMIT ?`
	HGDeleteIAMRolePermissions = `DELETE FROM iam_role_permissions WHERE role_id=?`
	// 仅 ops_engineer 内置角色可通过双人审批调整权限；按主键更新并保留数据库侧保护，其他内置角色仍由迁移维护。
	HGBumpIAMRoleVersion      = `UPDATE iam_roles SET version=version+1,updated_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status=1 AND (built_in=0 OR role_code='ops_engineer')`
	HGInsertIAMRolePermission = `INSERT INTO iam_role_permissions(role_id,permission_id) SELECT ?,id FROM iam_permissions WHERE permission_code=? LIMIT 1`
	HGSelectIAMOrganization   = `SELECT 1 FROM iam_organizations WHERE id=? AND org_type=? AND status=1 LIMIT 1`
	// uk_mall_merchant_id 唯一点查；仅审核通过的真实商家可作为未登记 shop 的授权作用域。
	// IAM 已登记作用域优先沿用旧协议，回退查询不写入表，也不把客户端 ID 当可信登记。
	HGSelectIAMApprovedShopScope = `SELECT 1 FROM mall_merchant WHERE merchant_id=? AND status=1 LIMIT 1`
)

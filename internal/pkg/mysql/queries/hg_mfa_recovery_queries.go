package SQLQueriesPackage

// 恢复邮箱按 user_id 主键点查、按 email 唯一约束去重；不扫描大表。
const (
	// 启动时只验证迁移列与访问权限，不读取、不修改业务数据。
	HGCheckRecoverySchema          = `SELECT e.session_generation,e.email,e.verified_at,e.available_at,c.purpose,c.digest,c.expires_at FROM user_mfa_recovery e JOIN user_mfa_recovery_challenge c ON c.user_id=e.user_id LIMIT 0`
	HGSelectRecoveryEmail          = `SELECT email,available_at,available_at<=UTC_TIMESTAMP(3) FROM user_mfa_recovery WHERE user_id=?`
	HGSelectRecoveryEmailForUpdate = HGSelectRecoveryEmail + ` FOR UPDATE`
	HGInsertRecoveryEmail          = `INSERT INTO user_mfa_recovery(user_id,email,verified_at,available_at) VALUES(?,?,UTC_TIMESTAMP(3),TIMESTAMPADD(SECOND,?,UTC_TIMESTAMP(3)))`
	HGUpdateRecoveryEmail          = `UPDATE user_mfa_recovery SET email=?,verified_at=UTC_TIMESTAMP(3),available_at=TIMESTAMPADD(SECOND,?,UTC_TIMESTAMP(3)) WHERE user_id=?`
	HGResetRecoveryMFA             = `DELETE FROM iam_mfa_factors WHERE user_id=? AND enabled=1`
	HGRevokeRecoverySessions       = `UPDATE user_mfa_recovery SET session_generation=session_generation+1 WHERE user_id=?`
	HGSelectRecoveryGeneration     = `SELECT COALESCE((SELECT session_generation FROM user_mfa_recovery WHERE user_id=u.user_id),0) FROM users u WHERE u.user_id=?`
	// users 唯一键、恢复表主键、IM 会话索引；单次 SQL 同时判定代际与强制下线。
	HGSelectAuthSessionAuthority = `SELECT COALESCE(r.session_generation,0),EXISTS(SELECT 1 FROM im_device d WHERE d.user_id=? AND d.session_id=? AND d.forced_logout_at IS NOT NULL) FROM users u LEFT JOIN user_mfa_recovery r ON r.user_id=u.user_id WHERE u.user_id=?`
	HGLockRecoveryAccount        = `SELECT password_hash,salt FROM users WHERE user_id=? FOR UPDATE`
	HGSaveRecoveryChallenge      = `INSERT INTO user_mfa_recovery_challenge(user_id,purpose,digest,expires_at) VALUES(?,?,?,TIMESTAMPADD(SECOND,300,UTC_TIMESTAMP(3))) ON DUPLICATE KEY UPDATE digest=VALUES(digest),expires_at=VALUES(expires_at)`
	HGConsumeRecoveryChallenge   = `DELETE FROM user_mfa_recovery_challenge WHERE user_id=? AND purpose=? AND digest=? AND expires_at>UTC_TIMESTAMP(3)`
	HGClearRecoveryChallenges    = `DELETE FROM user_mfa_recovery_challenge WHERE user_id=?`
	// idx_im_device_active 按用户和活跃窗口有界更新；锁住 users 行并在超过五台时回滚。
	HGRevokeRecoveryIMDevices = `UPDATE im_device SET revoked_at=NOW(3),forced_logout_at=NOW(3) WHERE user_id=? AND revoked_at IS NULL AND last_active_at>NOW(3)-INTERVAL 30 DAY ORDER BY last_active_at,id LIMIT 6`
	HGSelectIAMMFAEnabled     = `SELECT enabled FROM iam_mfa_factors WHERE user_id=? LIMIT 1`
)

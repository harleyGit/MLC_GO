package SQLQueriesPackage

// 恢复邮箱按 user_id 主键点查、按 email 唯一约束去重；不扫描大表。
const (
	HGSelectRecoveryEmail          = `SELECT email,verified_at,available_at FROM user_mfa_recovery WHERE user_id=?`
	HGSelectRecoveryEmailForUpdate = `SELECT email,available_at FROM user_mfa_recovery WHERE user_id=? FOR UPDATE`
	HGUpsertRecoveryEmail          = `INSERT INTO user_mfa_recovery(user_id,email,verified_at,available_at) VALUES(?,?,CURRENT_TIMESTAMP(3),?) ON DUPLICATE KEY UPDATE email=VALUES(email),verified_at=CURRENT_TIMESTAMP(3),available_at=VALUES(available_at)`
	HGResetRecoveryMFA             = `UPDATE iam_mfa_factors SET enabled=0,secret_cipher=?,last_counter=-1,updated_at=CURRENT_TIMESTAMP(3) WHERE user_id=? AND enabled=1`
	HGRevokeRecoverySessions       = `UPDATE users SET mfa_recovery_revoked_at=CURRENT_TIMESTAMP(3) WHERE user_id=?`
	HGSelectRecoveryRevokedAt      = `SELECT mfa_recovery_revoked_at FROM users WHERE user_id=? LIMIT 1`
	HGSelectIAMMFAEnabled          = `SELECT enabled FROM iam_mfa_factors WHERE user_id=? LIMIT 1`
)

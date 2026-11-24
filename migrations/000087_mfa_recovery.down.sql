ALTER TABLE users DROP COLUMN mfa_recovery_revoked_at;
DROP TABLE IF EXISTS user_mfa_recovery;

-- 恢复邮箱仅用于本人找回 MFA；邮箱唯一、按 user_id 点查，不回填历史账号的信任状态。
CREATE TABLE user_mfa_recovery (
  user_id VARCHAR(64) NOT NULL PRIMARY KEY,
  email VARCHAR(255) NOT NULL,
  verified_at DATETIME(3) NOT NULL,
  available_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_user_mfa_recovery_email (email)
) ENGINE=InnoDB COMMENT='已验证 MFA 恢复邮箱和冷却期';

-- 登录会话在恢复前签发的令牌一律失效；新增会话可以在冷却后重新登录。
ALTER TABLE users ADD COLUMN mfa_recovery_revoked_at DATETIME(3) NULL COMMENT 'MFA 找回时全部登录会话撤销时间';

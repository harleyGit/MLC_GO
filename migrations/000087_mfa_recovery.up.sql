-- 恢复邮箱仅用于本人找回 MFA；邮箱唯一、按 user_id 点查，不回填历史账号的信任状态。
CREATE TABLE user_mfa_recovery (
  user_id VARCHAR(255) NOT NULL PRIMARY KEY,
  email VARCHAR(254) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  verified_at DATETIME(3) NOT NULL,
  available_at DATETIME(3) NOT NULL,
  session_generation BIGINT UNSIGNED NOT NULL DEFAULT 0,
  UNIQUE KEY uk_user_mfa_recovery_email (email),
  FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='已验证恢复邮箱与会话代际';

-- 每账号、每用途最多一行；验证码仅保存带密钥摘要，消费与绑定/重置同事务。
CREATE TABLE user_mfa_recovery_challenge (
  user_id VARCHAR(255) NOT NULL,
  purpose TINYINT UNSIGNED NOT NULL,
  digest BINARY(32) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  PRIMARY KEY (user_id, purpose),
  FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='有界的一次性 MFA 恢复验证码';

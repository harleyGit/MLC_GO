-- 管理员直接修改他人安全字段时只记录字段名，绝不记录手机号、邮箱或密码值。
CREATE TABLE user_directory_security_audit (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    operator_user_id VARCHAR(255) NOT NULL,
    target_user_id VARCHAR(255) NOT NULL,
    changed_fields VARCHAR(128) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_user_directory_audit_target (target_user_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员修改他人账号安全字段审计';

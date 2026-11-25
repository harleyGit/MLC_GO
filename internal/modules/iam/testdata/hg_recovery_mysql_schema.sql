-- 从项目现有迁移截取恢复功能使用的列和索引；此文件仅供随机 Unix socket 测试实例。
CREATE DATABASE hg_recovery_fixture;
USE hg_recovery_fixture;

CREATE TABLE users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(255) NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    salt VARCHAR(64) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE iam_mfa_factors (
    user_id VARCHAR(64) NOT NULL PRIMARY KEY,
    secret_cipher VARBINARY(255) NOT NULL,
    enabled TINYINT(1) NOT NULL DEFAULT 0,
    last_counter BIGINT NOT NULL DEFAULT -1,
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB;

CREATE TABLE iam_audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    actor_user_id VARCHAR(64) NOT NULL,
    action VARCHAR(128) NOT NULL,
    target_type VARCHAR(64) NOT NULL,
    target_id VARCHAR(64) NOT NULL,
    request_id VARCHAR(64) NOT NULL,
    detail JSON NULL
) ENGINE=InnoDB;

CREATE TABLE im_device (
    id BINARY(16) NOT NULL PRIMARY KEY,
    user_id VARCHAR(128) NOT NULL,
    session_id BINARY(16) NULL,
    forced_logout_at DATETIME(3) NULL,
    last_active_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    revoked_at DATETIME(3) NULL,
    KEY idx_im_device_active (user_id,revoked_at,last_active_at,id),
    KEY idx_im_device_session_logout (user_id,session_id,forced_logout_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO users(user_id,password_hash,salt)
VALUES ('fixture-user', SHA2('fixture-passwordfixture-salt',256), 'fixture-salt'),
       ('fixture-other', SHA2('fixture-passwordfixture-salt',256), 'fixture-salt');
INSERT INTO im_device(id,user_id,session_id)
VALUES (UNHEX('00000000000000000000000000000001'), 'fixture-user', UNHEX('00000000000000000000000000000011')),
       (UNHEX('00000000000000000000000000000002'), 'fixture-other', UNHEX('00000000000000000000000000000022'));

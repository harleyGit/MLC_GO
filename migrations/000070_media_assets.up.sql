-- 上传仅落到隔离桶；审核封存、业务引用和清理尚未启用时不得将对象标记为 ready。
CREATE TABLE media_assets (
    asset_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
    owner_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    bucket VARCHAR(128) NOT NULL,
    object_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    mime_type VARCHAR(128) NOT NULL,
    file_size BIGINT UNSIGNED NOT NULL,
    sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    status VARCHAR(24) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_media_object (bucket, object_key),
    KEY idx_media_owner (owner_id, asset_id),
    KEY idx_media_status_created (status, created_at, asset_id)
) ENGINE=InnoDB;

CREATE TABLE media_upload_quota (
    owner_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY
) ENGINE=InnoDB;

CREATE TABLE media_upload_sessions (
    upload_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
    asset_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    owner_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    idempotency_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    mime_type VARCHAR(128) NOT NULL,
    expected_size BIGINT UNSIGNED NOT NULL,
    expected_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    object_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    status VARCHAR(24) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    completed_at DATETIME(3),
    created_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_media_owner_retry (owner_id, idempotency_key),
    UNIQUE KEY uk_media_session_asset (asset_id),
    KEY idx_media_owner_active (owner_id, status, expires_at),
    KEY idx_media_expired (status, expires_at, upload_id)
) ENGINE=InnoDB;

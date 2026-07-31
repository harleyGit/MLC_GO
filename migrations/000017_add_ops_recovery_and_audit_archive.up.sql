-- Timeout recovery scans one bounded oldest-first approving batch through this index.
ALTER TABLE `ops_coin_correction`
    ADD INDEX `idx_status_updated_id` (`status`, `updated_at`, `id`), ALGORITHM=INPLACE, LOCK=NONE;

-- Global time/id keyset supports bounded archive copy and retention verification without table scans.
ALTER TABLE `ops_asset_audit`
    ADD INDEX `idx_created_id` (`created_at`, `id`), ALGORITHM=INPLACE, LOCK=NONE;

-- Archive rows retain the source audit ID so repeated archive copies remain idempotent.
CREATE TABLE `ops_asset_audit_archive` LIKE `ops_asset_audit`;
ALTER TABLE `ops_asset_audit_archive`
    MODIFY COLUMN `id` BIGINT UNSIGNED NOT NULL,
    DROP PRIMARY KEY,
    ADD PRIMARY KEY (`id`);

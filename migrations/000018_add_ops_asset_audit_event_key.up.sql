-- Existing rows remain NULL because historical retries may already contain duplicates. All new application writes provide a stable non-NULL key.
ALTER TABLE `ops_asset_audit`
    ADD COLUMN `event_key` varchar(255) DEFAULT NULL AFTER `id`,
    ADD UNIQUE INDEX `uidx_event_key` (`event_key`), ALGORITHM=INPLACE, LOCK=NONE;

-- Keep archive column order aligned with the source because the bounded archive job uses INSERT ... SELECT *.
ALTER TABLE `ops_asset_audit_archive`
    ADD COLUMN `event_key` varchar(255) DEFAULT NULL AFTER `id`,
    ADD UNIQUE INDEX `uidx_event_key` (`event_key`), ALGORITHM=INPLACE, LOCK=NONE;

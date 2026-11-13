-- 仅用于经批准的回滚：先停用 worker、核实无待删除/审核任务；不自动删除 SeaweedFS 对象。
DROP TABLE IF EXISTS media_delete_jobs;
ALTER TABLE media_upload_sessions DROP KEY idx_media_review, DROP COLUMN lease_token, DROP COLUMN next_check_at, DROP COLUMN attempts;
ALTER TABLE media_assets DROP COLUMN duration_ms, DROP COLUMN height, DROP COLUMN width, DROP COLUMN sealed_key;

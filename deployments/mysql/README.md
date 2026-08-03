# MySQL Operations

## Asset Audit

- Online retention: 400 days.
- Archive retention: at least 2555 days (7 years), with deletion governed by the organization's legal hold policy.
- Run `ops_asset_audit_archive.sql.example` as a scheduled, single-owner job. Start with 1000 rows per transaction and reduce the batch if lock wait, transaction duration, binlog growth, or replica lag rises.
- Verify copied row count and checksums before allowing source deletion. Never run an unbounded `INSERT ... SELECT` or `DELETE`.
- Provision accounts with `ops_asset_audit_accounts.sql.example`; replace placeholders through DBA automation and Secret management. The reader receives only `SELECT`; the archiver is separate and receives only the table-specific privileges required for bounded copy/delete.
- Do not run migration 000016 down in an environment containing audit or correction records. Recovery and audit schema changes are roll-forward only.
- See `ops_asset_audit_event_key_rehearsal.md` for migration 000018 measurements, remaining production-like pre checks, and post-deployment conflict PromQL.

## Migration Rehearsal

- Match the production MySQL 8.0 minor version, SQL mode, charset, table statistics, parameter group, topology, and anonymized row/index scale.
- Historical migrations 000001-000009 use `USE HG_MLC_DB`; create/use that isolated schema during rehearsal instead of pointing the migration command at another schema name.
- Migration 000015 adds STORED generated columns. MySQL 8.0.44 rejects `ALGORITHM=INPLACE`; the migration uses `ALGORITHM=COPY`, so schedule a maintenance or online-schema-change window and observe metadata locks and table-copy space.
- Apply 14 -> 15 -> 16 -> 17 one version at a time. Record `schema_migrations.dirty`, DDL duration, `performance_schema.metadata_locks`, disk/redo/binlog growth, and replica lag after every step.
- Run `EXPLAIN ANALYZE` for the reprojection, consolidation, correction timeout, and audit archive queries with production-like cardinality before approval.

## Lot Consolidation Canary

- Production starts at one wallet per minute and at most four source lots per wallet.
- Observe `performance_schema.data_lock_waits`, `performance_schema.events_transactions_summary_global_by_event_name`, binlog bytes, and replica lag before increasing either bound.
- Stop the canary on recurring lock wait/deadlock errors, p99 transaction duration approaching the 20 second job timeout, sustained binlog growth above the normal write baseline, or replica lag above the service recovery objective.

## Video Comment Reaction Backfill

- Migration 000022 creates reaction shard tables but intentionally does not scan `video_comment_reactions`; schema migrations must not contain an unbounded `INSERT ... SELECT` for this table.
- Apply migration 000023 first, then stop or reject reaction writes before running the backfill. Running old relationship backfill while the new API also updates shards will double count.
- Run `go run ./cmd/hg_video_comment_reaction_backfill --env=pre --batch-size=10000 --pause=100ms` in pre first. The checkpoint and each primary-key range aggregation commit in one transaction, so restart resumes without repeating a committed batch.
- Start with one worker. Reduce `--batch-size` or increase `--pause` when redo/binlog growth, lock wait, transaction duration, or replica lag exceeds the pre baseline.
- Before reopening reaction writes, compare relationship aggregates and shard totals by `comment_id` and `CRC32(user_id) % 32`, then mark affected comments dirty for list/hot reprojection.
- Do not claim an online billion-row migration is safe until production-like pre rehearsal records runtime, temporary space, redo/binlog, replica lag, lock waits, and abort thresholds.

## Video Comment S3 Release Gate

- Export the production-equivalent `VIDEO_COMMENT_S3_*` values in the release job and run `MLC_S3_INTEGRATION=1 go test ./internal/pkg/upload -run TestS3StorageIntegrationPutCDNGetDelete -count=1`.
- The gate performs real PUT, bounded-retry CDN GET/content verification, and two signed storage DELETE calls using a unique probe key. A 2xx response confirms DELETE authorization and the second call confirms the idempotent retry contract without requiring bucket-list permission. It must run once per release environment, not once per application replica.
- Do not print access keys, secret keys, SigV4 authorization headers, or full credential-bearing configuration in release logs.

## Video Comment Operations

- Configure `video_comment.trusted_proxy_cidrs` or `VIDEO_COMMENT_TRUSTED_PROXY_CIDRS` with only the actual load balancer, ingress, and reverse-proxy networks. The API ignores forwarded headers from all other direct peers and rejects catch-all `/0` networks.
- Alert on `mlc_video_comment_reaction_dirty_oldest_age_seconds` and `mlc_video_comment_image_cleanup_oldest_age_seconds` exceeding the maintenance interval/SLO for multiple runs.
- Rate-alert on `mlc_video_comment_reaction_projection_cas_misses_total`, `mlc_video_comment_image_cleanup_expired_lease_reclaims_total`, and `mlc_video_comment_image_cleanup_failures_total`; sustained increases indicate hot-comment contention, worker crashes/timeouts, or object-storage permission/availability failures.

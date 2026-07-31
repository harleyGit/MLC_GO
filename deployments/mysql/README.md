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

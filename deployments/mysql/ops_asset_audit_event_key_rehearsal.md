# Ops Asset Audit Event Key Rehearsal

## 2026-07-31 Migration-Specific Rehearsal

- Environment: local pre simulation on MySQL `8.0.44`, single node, binary logging enabled with `ROW` format.
- Scope: isolated `HG_MLC_PRE_DB` schema containing the migration-17 audit table definitions and 100,000 synthetic audit rows.
- Command: `migrate -path migrations -database <redacted-pre-dsn> up 1`.
- Result: migration `18` completed with `dirty=0` in `1.318334708s`.
- Metadata locks: pending locks were `0` before and `0` immediately after the DDL. The short DDL interval was not continuously sampled, so transient granted locks are not quantified.
- Binary log: `binlog.000004:14252346` to `binlog.000004:14254217`, an increase of `1871` bytes.
- Table allocation: combined `DATA_LENGTH + INDEX_LENGTH` remained `38,469,632` bytes immediately before and after; refresh table statistics before using this value for capacity estimates.
- Schema verification: both audit tables contain nullable `event_key`; both contain unique `uidx_event_key` indexes.
- Replica lag: not measurable because this local pre simulation has no replica (`SHOW REPLICA STATUS` returned no rows).

## Production-Like Pre Requirement

The configured pre endpoint `127.0.0.1:3308` was initially unavailable. After starting the repository's local pre compose service, its existing `HG_MLC_DB` volume was at migration `3` and the historical `000004` migration could not proceed because `user_security` was absent. The attempted migration left version `4 dirty=1`; it was restored to the original `3 dirty=0` without modifying application tables.

Before production approval, repeat migration `17 -> 18` on the actual production-like pre topology and record:

- continuously sampled pending metadata locks during DDL;
- primary and replica binlog positions or GTID sets;
- peak and recovery replica lag;
- table and index size after refreshed statistics;
- application write latency and audit insert error rate.

## Post-Deployment Conflict Monitoring

The application exports only two fixed-source series:

```promql
mlc_ops_asset_audit_event_key_conflicts_total{source="correction_recovery"}
mlc_ops_asset_audit_event_key_conflicts_total{source="ops_api"}
```

Expected duplicate writes should predominantly originate from `correction_recovery`. Investigate request ID reuse or caller retry behavior whenever this query is positive:

```promql
increase(mlc_ops_asset_audit_event_key_conflicts_total{source="ops_api"}[15m]) > 0
```

Compare sources over a deployment window with:

```promql
sum by (source) (increase(mlc_ops_asset_audit_event_key_conflicts_total[24h]))
```

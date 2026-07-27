CREATE DATABASE IF NOT EXISTS mlc;

CREATE TABLE IF NOT EXISTS mlc.statistic_events
(
    event_id String,
    event_name LowCardinality(String),
    event_key String,
    submission_id String,
    user_id String,
    event_version UInt16,
    event_timestamp Int64,
    source_service LowCardinality(String),
    trace_id String,
    request_id String,
    kafka_topic LowCardinality(String),
    kafka_partition Int32,
    kafka_offset Int64,
    redis_generation LowCardinality(String),
    redis_shard UInt16,
    payload String,
    ingested_timestamp Int64
)
ENGINE = ReplacingMergeTree(ingested_timestamp)
PARTITION BY toYYYYMM(fromUnixTimestamp64Milli(event_timestamp))
ORDER BY (event_name, event_id)
TTL toDateTime(intDiv(event_timestamp, 1000), 'UTC') + INTERVAL 730 DAY DELETE;

CREATE TABLE IF NOT EXISTS mlc.statistic_event_totals
(
    redis_generation LowCardinality(String),
    redis_shard UInt16,
    event_name LowCardinality(String),
    event_ids AggregateFunction(uniqExact, String)
)
ENGINE = AggregatingMergeTree
ORDER BY (redis_generation, redis_shard, event_name);

CREATE MATERIALIZED VIEW IF NOT EXISTS mlc.statistic_event_totals_mv
TO mlc.statistic_event_totals
AS
SELECT
    redis_generation,
    redis_shard,
    event_name,
    uniqExactState(event_id) AS event_ids
FROM mlc.statistic_events
GROUP BY redis_generation, redis_shard, event_name;

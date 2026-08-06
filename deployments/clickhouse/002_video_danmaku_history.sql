CREATE DATABASE IF NOT EXISTS mlc;

CREATE TABLE IF NOT EXISTS mlc.video_danmaku_history
(
    danmaku_id String,
    submission_id String,
    video_id String,
    user_id String,
    request_id String,
    content String,
    progress_ms UInt32,
    mode LowCardinality(String),
    color FixedString(7),
    font_size UInt8,
    created_at Int64,
    kafka_topic LowCardinality(String),
    kafka_partition Int32,
    kafka_offset Int64,
    ingested_at Int64
)
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(fromUnixTimestamp64Milli(created_at))
ORDER BY (video_id, progress_ms, created_at, danmaku_id)
TTL toDateTime(intDiv(created_at, 1000), 'UTC') + INTERVAL 1825 DAY DELETE;

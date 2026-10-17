-- 在目标ClickHouse数据库内执行；不自动执行。累计统计不可跨快照求和。
CREATE TABLE IF NOT EXISTS crawler_video_stats_history (
    collection_id String COMMENT '稳定采集UUID，重放保持不变',
    bvid String COMMENT 'Bilibili视频唯一标识',
    collected_at Int64 COMMENT 'UTC Unix毫秒，同次采集分区时间固定',
    view_count UInt64 COMMENT '累计播放次数',
    like_count UInt64 COMMENT '累计点赞次数',
    coin_count UInt64 COMMENT '累计投币枚数',
    favorite_count UInt64 COMMENT '累计收藏次数',
    share_count UInt64 COMMENT '累计分享次数',
    reply_count UInt64 COMMENT '累计评论条数',
    danmaku_count UInt64 COMMENT '累计弹幕条数'
) ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(fromUnixTimestamp64Milli(collected_at))
ORDER BY (bvid, collected_at, collection_id)
TTL toDateTime(fromUnixTimestamp64Milli(collected_at)) + INTERVAL 180 DAY;

-- 合并前也按(bvid,collected_at,collection_id)分组去重；查询必须限定单视频与时间窗，限制结果数。
-- SELECT collected_at,collection_id,any(view_count) AS view_count
-- FROM crawler_video_stats_history
-- WHERE bvid={bvid:String} AND collected_at >= {since:Int64} AND collected_at < {until:Int64}
-- GROUP BY bvid,collected_at,collection_id ORDER BY collected_at DESC,collection_id DESC LIMIT 1000;

-- 仅在停写并确认不再需要采集详情与状态后执行；将删除已采集数据。
DROP TABLE IF EXISTS crawler_video_stats_latest;
DROP TABLE IF EXISTS crawler_video_details;
DROP TABLE IF EXISTS crawler_video_collections;

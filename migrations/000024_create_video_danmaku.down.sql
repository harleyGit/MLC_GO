-- 先删除计数派生表，再删除权威热表；回滚会永久删除弹幕数据，生产执行前必须完成备份和流量切断。
DROP TABLE IF EXISTS `video_danmaku_stat_shards`;
DROP TABLE IF EXISTS `video_danmaku`;

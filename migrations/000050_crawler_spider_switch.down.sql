-- 回滚将删除开关状态与审计记录；执行前需备份并明确授权。
DROP TABLE IF EXISTS crawler_spider_switch_events;
DROP TABLE IF EXISTS crawler_spider_switches;

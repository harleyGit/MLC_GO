-- 仅控制独立 hg_crawler 的 Bilibili 周期推荐 worker；不控制 cron 任务或详情 Kafka 消费者。
-- 先迁移，再发布主服务与独立 crawler；默认停止，防止升级时意外自动抓取。
CREATE TABLE crawler_spider_switches (
    spider_id VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '固定平台 worker 标识',
    desired_running TINYINT(1) NOT NULL DEFAULT 0 COMMENT '数据库权威期望状态：1运行/0停止',
    updated_by VARCHAR(128) NOT NULL DEFAULT '' COMMENT '最近一次操作的用户ID，不保存令牌',
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '最近一次切换时间 UTC',
    PRIMARY KEY (spider_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='独立推荐爬虫期望运行状态';

CREATE TABLE crawler_spider_switch_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '审计游标',
    spider_id VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'worker 标识',
    desired_running TINYINT(1) NOT NULL COMMENT '本次切换后的期望状态',
    actor_id VARCHAR(128) NOT NULL COMMENT '发起操作的用户ID',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '切换提交时间 UTC',
    PRIMARY KEY (id),
    KEY idx_crawler_spider_switch_events_spider (spider_id, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='推荐爬虫每次成功切换的操作记录';

INSERT INTO crawler_spider_switches (spider_id, desired_running) VALUES ('bilibili', 0);

-- 新增表不改写既有大表；发布前先执行本迁移，再启用采集流水线。
CREATE TABLE crawler_video_details (
    bvid VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'Bilibili视频唯一标识',
    snapshot JSON NOT NULL COMMENT '清洗后的基础详情及有界分P快照，不含累计统计',
    collected_at DATETIME(6) NOT NULL COMMENT 'UTC采集时间，微秒精度，旧快照不得覆盖新值',
    collection_id CHAR(36) CHARACTER SET ascii NOT NULL COMMENT '本次采集UUID，同时间戳时用于稳定版本比较',
    PRIMARY KEY (bvid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='外部视频基础详情及分P快照';

CREATE TABLE crawler_video_stats_latest (
    bvid VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'Bilibili视频唯一标识，与详情一对一',
    snapshot JSON NOT NULL COMMENT '最新播放点赞投币收藏分享评论弹幕累计值，禁止累加',
    collected_at DATETIME(6) NOT NULL COMMENT 'UTC采集时间，微秒精度',
    collection_id CHAR(36) CHARACTER SET ascii NOT NULL COMMENT '稳定采集UUID，与详情版本一致',
    PRIMARY KEY (bvid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='原站互动最新快照';

CREATE TABLE crawler_video_collections (
    collection_id CHAR(36) CHARACTER SET ascii NOT NULL COMMENT '单次采集UUID，重放幂等主键',
    bvid VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'Bilibili视频标识',
    task_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '任务定义ID，0表示周期推荐采集',
    run_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '任务执行ID，0表示周期推荐采集',
    status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending待抓取/raw待清洗/cleaned待入库/stored已入库/failed失败',
    error_message VARCHAR(255) NOT NULL DEFAULT '' COMMENT '不含原始响应与凭据的失败摘要',
    discovered_at DATETIME(6) NOT NULL COMMENT 'UTC发现时间，微秒精度',
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '数据库会话UTC更新时间，微秒精度',
    PRIMARY KEY (collection_id),
    KEY idx_video_recent (bvid, discovered_at DESC, collection_id DESC),
    KEY idx_run (run_id, collection_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='采集流水线状态与幂等边界，保留期覆盖消息重放窗口';

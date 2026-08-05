-- 视频弹幕是按 video_id 和播放时间读取的超大写多读表。这里不建立外键：
-- 1. 亿级热表上的外键检查和级联会放大写延迟、锁等待和后续分库分表成本；
-- 2. 视频存在性、公开状态和 close_danmaku 由写入事务在 video_files/video_submissions 唯一键上校验；
-- 3. 主键 id 仅用于库内稳定排序和未来归档，业务接口使用不可预测的 danmaku_id。
CREATE TABLE `video_danmaku` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `danmaku_id` VARCHAR(64) NOT NULL COMMENT '弹幕业务ID',
    `submission_id` VARCHAR(64) NOT NULL COMMENT '冗余稿件ID，便于审核和归档',
    `video_id` VARCHAR(64) NOT NULL COMMENT '具体分P视频ID，也是未来分片路由键',
    `user_id` VARCHAR(255) NOT NULL COMMENT '作者用户ID，仅服务端从认证上下文写入',
    `request_id` VARCHAR(64) NOT NULL COMMENT '用户维度幂等请求ID',
    `progress_ms` INT UNSIGNED NOT NULL COMMENT '弹幕对应播放位置，单位毫秒',
    `content` VARCHAR(100) NOT NULL COMMENT '弹幕文本，服务限制1到100字符',
    `mode` ENUM('scroll', 'top', 'bottom') NOT NULL DEFAULT 'scroll' COMMENT '滚动、顶部、底部',
    `color` CHAR(7) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '#FFFFFF' COMMENT '规范化RGB颜色',
    `font_size` TINYINT UNSIGNED NOT NULL DEFAULT 25 COMMENT '字体像素，服务限制12到36',
    `status` ENUM('active', 'deleted', 'blocked') NOT NULL DEFAULT 'active' COMMENT '展示和审核状态',
    `created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    `deleted_at` TIMESTAMP(6) NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_video_danmaku_id` (`danmaku_id`),
    UNIQUE KEY `uk_video_danmaku_user_request` (`user_id`, `request_id`),
    -- history 查询固定为 video_id + active + 有界 progress_ms，并按 progress_ms,id keyset 前进。
    KEY `idx_video_danmaku_timeline` (`video_id`, `status`, `progress_ms`, `id`),
    -- 审核/用户删除入口按用户和时间游标扫描，不允许对热表执行 offset 深分页。
    KEY `idx_video_danmaku_user_created` (`user_id`, `created_at` DESC, `id` DESC),
    KEY `idx_video_danmaku_submission_created` (`submission_id`, `created_at` DESC, `id` DESC),
    CONSTRAINT `chk_video_danmaku_font_size` CHECK (`font_size` BETWEEN 12 AND 36)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频弹幕权威热表';

-- 热门视频不能在每条弹幕写入时更新同一计数行，否则 InnoDB 行锁会把并发写串行化。
-- 64 个固定分片把写竞争摊开；读取总数最多聚合 64 行，成本与弹幕总量无关。
CREATE TABLE `video_danmaku_stat_shards` (
    `video_id` VARCHAR(64) NOT NULL,
    `shard_id` TINYINT UNSIGNED NOT NULL,
    `danmaku_count` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (`video_id`, `shard_id`),
    CONSTRAINT `chk_video_danmaku_stat_shard` CHECK (`shard_id` < 64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='视频弹幕64分片权威计数';

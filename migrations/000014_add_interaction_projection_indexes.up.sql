-- 四个索引只服务 MySQL -> Redis 的 updated_at 复合 keyset 扫描；worker SQL 使用 FORCE INDEX，
-- 因此必须先完成本 migration 再启用 interaction_reproject。
-- LOCK=NONE 明确要求在线写入不中断；若目标 MySQL 版本或表结构不支持，应让 migration 失败并改用
-- 受控 online schema change，不能静默退化为阻塞大表写入。
ALTER TABLE `video_user_interactions`
    ADD INDEX `idx_video_interaction_projection` (`updated_at`, `id`), ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE `user_follow_relations`
    ADD INDEX `idx_follow_projection` (`updated_at`, `id`), ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE `video_interaction_stat_shards`
    ADD INDEX `idx_video_count_projection` (`updated_at`, `submission_id`, `shard_id`), ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE `user_follow_stat_shards`
    ADD INDEX `idx_follow_count_projection` (`updated_at`, `user_id`, `shard_id`), ALGORITHM=INPLACE, LOCK=NONE;

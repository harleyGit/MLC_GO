ALTER TABLE `user_follow_stat_shards`
    DROP INDEX `idx_follow_count_projection`;

ALTER TABLE `video_interaction_stat_shards`
    DROP INDEX `idx_video_count_projection`;

ALTER TABLE `user_follow_relations`
    DROP INDEX `idx_follow_projection`;

ALTER TABLE `video_user_interactions`
    DROP INDEX `idx_video_interaction_projection`;

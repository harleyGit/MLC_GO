-- 旧 worker 不会领取 deleting；回滚字段前先转为不可绑定的 delete_pending，避免对象和配额永久滞留。
UPDATE `video_comment_images`
SET `status` = 'delete_pending', `delete_after` = CURRENT_TIMESTAMP(6)
WHERE `status` = 'deleting';

ALTER TABLE `video_comment_images`
    DROP KEY `idx_video_comment_images_deleting_lease`,
    DROP KEY `idx_video_comment_images_delete_cleanup`,
    DROP KEY `idx_video_comment_images_pending_cleanup`,
    DROP COLUMN `cleanup_retry_count`,
    DROP COLUMN `cleanup_lease_until`,
    DROP COLUMN `cleanup_token`;

ALTER TABLE `video_comment_reaction_dirty`
    DROP COLUMN `revision`,
    MODIFY COLUMN `updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6);

-- video_comment_reaction_backfill_state 故意不删除：回滚期间可能已有部分批次提交，保留 checkpoint 才能安全重新升级。

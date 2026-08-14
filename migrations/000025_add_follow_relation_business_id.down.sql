ALTER TABLE `user_follow_relations`
    DROP INDEX `uk_follow_relation_id`,
    DROP COLUMN `relation_id`;

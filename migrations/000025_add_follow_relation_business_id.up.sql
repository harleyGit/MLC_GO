ALTER TABLE `user_follow_relations`
    ADD COLUMN `relation_id` CHAR(12) CHARACTER SET ascii COLLATE ascii_bin NULL
        COMMENT '关注关系业务ID，O+Base62 Snowflake' AFTER `id`,
    ADD UNIQUE KEY `uk_follow_relation_id` (`relation_id`);

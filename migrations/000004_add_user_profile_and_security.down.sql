USE HG_MLC_DB;

DROP TABLE IF EXISTS `user_security`;

ALTER TABLE `users`
    DROP COLUMN `avatar_url`,
    DROP COLUMN `birth_month`,
    DROP COLUMN `gender`,
    DROP COLUMN `signature`,
    DROP COLUMN `nickname`;

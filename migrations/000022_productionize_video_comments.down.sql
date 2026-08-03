DELETE FROM `video_comment_reactions` WHERE `reaction` = 'none';
ALTER TABLE `video_comment_reactions`
    MODIFY COLUMN `reaction` ENUM('like', 'dislike') NOT NULL;
DROP TABLE IF EXISTS `video_comment_images`;
DROP TABLE IF EXISTS `video_comment_image_quotas`;
DROP TABLE IF EXISTS `video_comment_reaction_dirty`;
DROP TABLE IF EXISTS `video_comment_reaction_shards`;

-- 已有业务实体及授权后不可删除验证位，否则会使原有效绑定失效并导致候选目录不一致。
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='business scope candidate index rollback requires manual review';

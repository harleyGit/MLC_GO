-- 六类业务实体可能已关联 IAM 授权、任务、稿件和商家；禁止自动删除或静默回填。
-- 回退须先受控核对绑定、关联和业务调用方，再制定人工迁移与回退方案。
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='business scope entities rollback requires manual review';

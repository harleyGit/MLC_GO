USE HG_MLC_DB;

-- 资源登记与历史授权不能自动逆转；发布系统回滚到不支持新作用域的旧版本前，
-- 须先核对并处置六类绑定，再人工制定迁移方案。拒绝自动删除权威资源。
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='IAM scope resources rollback requires manual review';

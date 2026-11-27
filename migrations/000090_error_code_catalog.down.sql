-- 仅回退目录，不操作业务表；回滚应用无需执行本迁移。
DROP TABLE IF EXISTS hg_error_code_catalog;

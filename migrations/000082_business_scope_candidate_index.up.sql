-- 历史 IAM 占位资源不具备业务权威实体，不自动标记为已验证；新实体事务将此位设为 1。
-- 亿级候选查询必须先通过 (类型,状态,已验证,游标) 收窄，避免逐条探测占位记录造成深扫描。
-- 执行前在实际数据规模上评估 ADD COLUMN/ADD KEY 的元数据锁、redo/binlog、复制延迟与在线 DDL 能力。
ALTER TABLE iam_scope_resources
  ADD COLUMN business_verified TINYINT NOT NULL DEFAULT 0 COMMENT '0 仅 IAM 占位记录，1 同事务建立了六类权威业务实体；仍需点查业务状态',
  ADD KEY idx_iam_scope_business_page(scope_type,status,business_verified,id);

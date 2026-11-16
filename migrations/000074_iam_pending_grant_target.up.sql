-- 待审批授权按目标管理员分页查询：仅由 grant 申请 JSON 提取 userId，其他类型为空。
-- 依赖 iam_approval_requests 已存在；新增索引需评估在线 DDL 锁、磁盘占用及复制延迟，不能直接在未知或共享库执行。
ALTER TABLE iam_approval_requests
  ADD COLUMN target_user_id VARCHAR(64) GENERATED ALWAYS AS (
    CASE WHEN action = 'grant' THEN JSON_UNQUOTE(JSON_EXTRACT(payload, '$.userId')) ELSE NULL END
  ) VIRTUAL COMMENT 'grant 审批目标管理员业务用户 ID；非 grant 或缺失 userId 时为空，仅用于只读查询',
  ADD KEY idx_iam_approval_target_status_id (target_user_id, status, id);

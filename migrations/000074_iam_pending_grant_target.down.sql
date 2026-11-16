-- 回退目标用户待审批查询依赖后，移除索引及派生列；不修改申请记录。
ALTER TABLE iam_approval_requests
  DROP KEY idx_iam_approval_target_status_id,
  DROP COLUMN target_user_id;

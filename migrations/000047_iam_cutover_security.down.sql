-- 旧 ops 表保留供人工核对；回滚仅移除新增安全表，IAM 回填由 000046 的回滚处理。
DROP TABLE IF EXISTS iam_approval_requests;
DROP TABLE IF EXISTS iam_mfa_factors;
DROP TABLE IF EXISTS iam_policy_revision;

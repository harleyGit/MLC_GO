-- 回退仅适用于 IAM 投递工作者停机且已确认队列全部处理；删除会丢失待投递通知。
ALTER TABLE iam_user_role_bindings DROP KEY idx_iam_binding_role_user;
DROP TABLE iam_access_delivery_jobs;

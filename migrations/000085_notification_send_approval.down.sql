-- 仅用于尚未创建新状态任务的灰度回退；有 pending_approval 的数据时禁止执行回退。
DELETE rp FROM iam_role_permissions rp JOIN iam_permissions p ON p.id=rp.permission_id
WHERE p.permission_code='notification:send:approve';
DELETE FROM iam_permissions WHERE permission_code='notification:send:approve';
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
DROP TABLE notification_send_audit;
DROP TABLE notification_template_audit;
ALTER TABLE notification_send_tasks
  DROP KEY idx_notification_send_review,
  DROP COLUMN approved_at,
  DROP COLUMN approved_by,
  DROP COLUMN scheduled_at;

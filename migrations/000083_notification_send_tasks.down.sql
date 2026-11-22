START TRANSACTION;
DELETE rp FROM iam_role_permissions rp JOIN iam_permissions p ON p.id=rp.permission_id
WHERE p.permission_code IN ('notification:template:publish','notification:send:create','notification:send:read','notification:send:cancel');
DELETE FROM iam_permissions WHERE permission_code IN ('notification:template:publish','notification:send:create','notification:send:read','notification:send:cancel');
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;
DROP TABLE notification_send_tasks;

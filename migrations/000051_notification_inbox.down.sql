START TRANSACTION;
DELETE rp FROM iam_role_permissions rp JOIN iam_permissions p ON p.id=rp.permission_id
WHERE p.permission_code IN ('notification:channel:read','notification:delivery:read','notification:channel:send');
DELETE FROM iam_permissions WHERE permission_code IN ('notification:channel:read','notification:delivery:read','notification:channel:send');
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;
DROP TABLE notification_preferences;
DROP TABLE notification_inbox;

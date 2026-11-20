START TRANSACTION;
DELETE rp FROM iam_role_permissions rp JOIN iam_permissions p ON p.id=rp.permission_id
WHERE p.permission_code IN ('notification:template:read','notification:template:create','notification:template:update','notification:template:delete');
DELETE FROM iam_permissions WHERE permission_code IN ('notification:template:read','notification:template:create','notification:template:update','notification:template:delete');
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;
DROP TABLE notification_template_versions;
DROP TABLE notification_templates;

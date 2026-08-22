DELETE rp FROM `role_permission` rp JOIN `permission` p ON p.`id` = rp.`permission_id`
WHERE p.`code` LIKE 'crawler.task.%' AND rp.`create_by` = 'migration-000030';
DELETE FROM `permission` WHERE `code` LIKE 'crawler.task.%' AND `update_by` = 'migration-000030';

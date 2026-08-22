-- Crawler task administration uses the existing database-backed Ops RBAC boundary.
INSERT IGNORE INTO `permission` (`code`,`type`,`name`,`page_path`,`parent_id`,`status`,`sort`,`desc`,`create_at`,`update_at`,`update_by`) VALUES
('crawler.task.read',2,'Read crawler tasks','',-1,1,30,'Read crawler task definitions and run summaries',NOW(),NOW(),'migration-000030'),
('crawler.task.write',2,'Write crawler tasks','',-1,1,31,'Create and update crawler task definitions',NOW(),NOW(),'migration-000030'),
('crawler.task.run',2,'Run crawler tasks','',-1,1,32,'Debug and execute crawler tasks',NOW(),NOW(),'migration-000030');

INSERT IGNORE INTO `role_permission` (`role_id`,`permission_id`,`create_by`,`update_by`)
SELECT r.`id`, p.`id`, 'migration-000030', 'migration-000030'
FROM `role` r JOIN `permission` p ON p.`code` LIKE 'crawler.task.%' AND p.`status` = 1
WHERE r.`name` = 'super-admin' AND r.`status` = 1;

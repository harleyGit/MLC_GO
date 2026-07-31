-- Asset control-plane permissions. Compatibility bootstrap is intentionally restricted to an existing active role named exactly super-admin.
INSERT IGNORE INTO `permission` (`code`,`type`,`name`,`page_path`,`parent_id`,`status`,`sort`,`desc`,`create_at`,`update_at`,`update_by`) VALUES
('asset.coin.balance.read',2,'Read coin balance','',-1,1,1,'Read authoritative coin balance',NOW(),NOW(),'migration-000016'),
('asset.coin.transaction.read',2,'Read coin transactions','',-1,1,2,'Read authoritative coin transactions',NOW(),NOW(),'migration-000016'),
('asset.coin.grant',2,'Grant coins','',-1,1,3,'Grant coins through ops control plane',NOW(),NOW(),'migration-000016'),
('asset.coin.refund',2,'Refund coins','',-1,1,4,'Refund an original debit',NOW(),NOW(),'migration-000016'),
('asset.coin.correction.request',2,'Request coin correction','',-1,1,5,'Create a pending coin correction',NOW(),NOW(),'migration-000016'),
('asset.coin.correction.approve',2,'Approve coin correction','',-1,1,6,'Approve a pending coin correction',NOW(),NOW(),'migration-000016'),
('asset.coin.correction.apply',2,'Apply coin correction','',-1,1,7,'Apply an approved coin correction',NOW(),NOW(),'migration-000016'),
('asset.pipeline.read',2,'Read asset pipeline','',-1,1,8,'Read bounded asset pipeline status',NOW(),NOW(),'migration-000016'),
('ops.rbac.manage',2,'Manage ops RBAC','',-1,1,9,'Assign operation permissions to roles',NOW(),NOW(),'migration-000016');

INSERT IGNORE INTO `role_permission` (`role_id`,`permission_id`,`create_by`,`update_by`)
SELECT r.`id`, p.`id`, 'migration-000016', 'migration-000016'
FROM `role` r JOIN `permission` p ON p.`code` LIKE 'asset.%' AND p.`status` = 1
WHERE r.`name` = 'super-admin' AND r.`status` = 1;

CREATE TABLE `ops_coin_correction` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `correction_id` varchar(64) NOT NULL,
  `user_id` varchar(255) NOT NULL,
  `request_id` varchar(128) NOT NULL,
  `ticket_id` varchar(128) NOT NULL,
  `work_order_id` varchar(128) NOT NULL DEFAULT '',
  `delta` bigint NOT NULL,
  `reason` varchar(255) NOT NULL,
  `applicant_id` varchar(255) NOT NULL,
  `approver_id` varchar(255) NOT NULL DEFAULT '',
  `source_ip` varchar(64) NOT NULL,
  `tid` varchar(128) NOT NULL DEFAULT '',
  `status` varchar(16) NOT NULL,
  `transaction_id` bigint unsigned DEFAULT NULL,
  `balance_after` bigint unsigned DEFAULT NULL,
  `error_message` varchar(500) NOT NULL DEFAULT '',
  `approved_at` datetime(6) DEFAULT NULL,
  `applied_at` datetime(6) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uidx_correction_id` (`correction_id`),
  UNIQUE KEY `uidx_request_id` (`request_id`),
  KEY `idx_status_id` (`status`,`id`),
  KEY `idx_applicant_id` (`applicant_id`,`id`),
  CONSTRAINT `chk_ops_correction_delta` CHECK (`delta` <> 0),
  CONSTRAINT `chk_ops_correction_approver` CHECK (`approver_id` = '' OR `approver_id` <> `applicant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Two-step coin correction workflow';

CREATE TABLE `ops_asset_audit` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `operator_id` varchar(255) NOT NULL,
  `action` varchar(64) NOT NULL,
  `target_user_id` varchar(255) NOT NULL DEFAULT '',
  `source_ip` varchar(64) NOT NULL,
  `request_id` varchar(128) NOT NULL DEFAULT '',
  `tid` varchar(128) NOT NULL DEFAULT '',
  `old_balance` bigint unsigned NOT NULL DEFAULT 0,
  `new_balance` bigint unsigned NOT NULL DEFAULT 0,
  `applicant_id` varchar(255) NOT NULL DEFAULT '',
  `approver_id` varchar(255) NOT NULL DEFAULT '',
  `outcome` varchar(32) NOT NULL,
  `error_message` varchar(500) NOT NULL DEFAULT '',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  KEY `idx_operator_created` (`operator_id`,`created_at`,`id`),
  KEY `idx_target_created` (`target_user_id`,`created_at`,`id`),
  KEY `idx_request_id` (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Immutable ops asset audit log';

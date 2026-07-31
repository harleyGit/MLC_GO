DROP TABLE IF EXISTS `ops_asset_audit`;
DROP TABLE IF EXISTS `ops_coin_correction`;
DELETE rp FROM `role_permission` rp JOIN `permission` p ON p.`id` = rp.`permission_id` WHERE (p.`code` LIKE 'asset.%' OR p.`code` = 'ops.rbac.manage') AND rp.`create_by` = 'migration-000016';
DELETE FROM `permission` WHERE `code` IN ('asset.coin.balance.read','asset.coin.transaction.read','asset.coin.grant','asset.coin.refund','asset.coin.correction.request','asset.coin.correction.approve','asset.coin.correction.apply','asset.pipeline.read','ops.rbac.manage') AND `update_by` = 'migration-000016';

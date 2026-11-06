-- 回滚索引需在停止依赖该索引的用户删除入口后执行。
START TRANSACTION;
DELETE rp FROM iam_role_permissions rp JOIN iam_permissions p ON p.id=rp.permission_id WHERE p.permission_code='user:directory:delete';
DELETE FROM iam_permissions WHERE permission_code='user:directory:delete';
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;
ALTER TABLE im_call_session DROP KEY idx_im_call_callee;
ALTER TABLE resource_upload_files DROP KEY idx_resource_files_user;
ALTER TABLE order_items DROP KEY idx_order_items_user;
ALTER TABLE im_friend_remark DROP KEY idx_im_friend_remark_friend;
ALTER TABLE im_realtime_job DROP KEY idx_im_job_user;
ALTER TABLE im_conversation DROP KEY idx_im_conversation_high;
ALTER TABLE notification_event_ledger DROP KEY idx_notify_event_receiver;
ALTER TABLE crawler_spider_switch_events DROP KEY idx_crawler_spider_actor;
ALTER TABLE user_directory_security_audit DROP KEY idx_user_directory_audit_operator;
ALTER TABLE mall_product_edit DROP KEY idx_mall_edit_user;
ALTER TABLE iam_approval_requests DROP KEY idx_iam_approval_approver;
ALTER TABLE video_comment_reactions DROP KEY idx_comment_reaction_user;
ALTER TABLE video_comments DROP KEY idx_video_comments_reply_user;
ALTER TABLE ops_coin_correction DROP KEY idx_ops_correction_user, DROP KEY idx_ops_correction_approver;
ALTER TABLE ops_asset_audit DROP KEY idx_ops_asset_audit_applicant, DROP KEY idx_ops_asset_audit_approver;
-- 删除审计属于不可恢复操作的记录；回滚服务代码不自动销毁审计表，需数据保留方案单独审批。

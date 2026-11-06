-- 删除用户的点查必须全部命中前缀索引；发布顺序：先索引，后服务与前端。
-- 大表索引需在隔离环境按实际数据量评估 DDL、复制延迟、存储与回滚窗口；不得在业务高峰直接执行。
CREATE TABLE user_directory_delete_audit (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    operator_user_id VARCHAR(255) NOT NULL,
    target_user_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_user_delete_audit_operator (operator_user_id, id),
    KEY idx_user_delete_audit_target (target_user_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员删除无关联账号审计；与删除同事务提交';
ALTER TABLE ops_coin_correction ADD KEY idx_ops_correction_user (user_id), ADD KEY idx_ops_correction_approver (approver_id);
ALTER TABLE ops_asset_audit ADD KEY idx_ops_asset_audit_applicant (applicant_id), ADD KEY idx_ops_asset_audit_approver (approver_id);
ALTER TABLE video_comments ADD KEY idx_video_comments_reply_user (reply_to_user_id);
ALTER TABLE video_comment_reactions ADD KEY idx_comment_reaction_user (user_id);
ALTER TABLE mall_product_edit ADD KEY idx_mall_edit_user (user_id);
ALTER TABLE iam_approval_requests ADD KEY idx_iam_approval_approver (approver_id);
ALTER TABLE user_directory_security_audit ADD KEY idx_user_directory_audit_operator (operator_user_id);
ALTER TABLE crawler_spider_switch_events ADD KEY idx_crawler_spider_actor (actor_id);
ALTER TABLE notification_event_ledger ADD KEY idx_notify_event_receiver (receiver_id);
ALTER TABLE im_conversation ADD KEY idx_im_conversation_high (user_high);
ALTER TABLE im_realtime_job ADD KEY idx_im_job_user (user_id);
ALTER TABLE im_friend_remark ADD KEY idx_im_friend_remark_friend (friend_user_id);
ALTER TABLE im_call_session ADD KEY idx_im_call_callee (callee_id);
ALTER TABLE order_items ADD KEY idx_order_items_user (user_id);
ALTER TABLE resource_upload_files ADD KEY idx_resource_files_user (user_id);
START TRANSACTION;
INSERT INTO iam_permissions(permission_code,description) VALUES ('user:directory:delete','删除无关联且无有效会话的用户');
-- 仅向内置超级管理员引导授权；不会把跨用户编辑权限隐式升级为删除权限。
INSERT IGNORE INTO iam_role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM iam_roles r JOIN iam_permissions p ON p.permission_code='user:directory:delete'
WHERE r.role_code='super_admin' AND r.status=1;
UPDATE iam_policy_revision SET revision=revision+1 WHERE id=1;
COMMIT;

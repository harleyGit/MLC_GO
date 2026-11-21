-- IAM 授权事务仅写一个有界投递任务；不在事务内遍历亿级绑定或连接实时网关。
CREATE TABLE iam_access_delivery_jobs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '按提交后可见的主键领用任务，不用主键作为授权版本',
  target_type VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'user 表示单个用户，role 表示按角色绑定游标分批投递',
  target_id VARCHAR(64) NOT NULL COMMENT 'user_id 或角色 ID 十进制字符串，不允许由客户端提交',
  role_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '操作时角色显示名称，可空，发送时从权威角色表补齐',
  last_user_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'role 任务的用户标识游标，空串表示从头开始',
  status VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending' COMMENT 'pending 等待/重试，done 已完成',
  lease_token CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '工作者随机租约标识，空串表示无人领用',
  available_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '下一次领取时间，毫秒精度，按数据库会话时区',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT 'IAM 原事务写入时间，毫秒精度，按数据库会话时区',
  PRIMARY KEY(id),
  KEY idx_iam_delivery_pending(status,available_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='IAM 事务性权限变更投递队列；完成后由运维周期归档';

-- 已有角色绑定索引以 scope 列居中，不适合按角色用户游标扫描；DDL 上线前评估大表索引构建锁和复制延迟。
ALTER TABLE iam_user_role_bindings ADD KEY idx_iam_binding_role_user (role_id,user_id,status);

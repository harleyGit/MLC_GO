USE HG_MLC_DB;

-- 六类新作用域仅登记 IAM 资源，不自动改变业务 API 的鉴权规则；业务端须传入服务端权威 ID。
-- 候选页使用 (scope_type,status,id) 游标；授权点查使用唯一键，不以客户端输入推断资源。
CREATE TABLE iam_scope_resources (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '候选列表游标主键',
  scope_type VARCHAR(32) NOT NULL COMMENT '六类已接入 IAM 资源登记的作用域类型',
  scope_id VARCHAR(64) NOT NULL COMMENT '服务端生成的不可变资源 ID',
  scope_name VARCHAR(128) NOT NULL COMMENT '管理端展示名称',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1 可用于授权，0 停用',
  created_by VARCHAR(64) NOT NULL COMMENT '登记的管理员用户 ID',
  request_id VARCHAR(64) NOT NULL COMMENT '管理员资源登记的幂等请求 ID',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY(id), UNIQUE KEY uk_iam_scope_resource(scope_type,scope_id),
  UNIQUE KEY uk_iam_scope_resource_request(created_by,request_id),
  KEY idx_iam_scope_resource_page(scope_type,status,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='新作用域 IAM 资源登记，不代替业务权威实体';

-- 仅在资源登记和服务端授权校验部署后应用；未创建资源时，前端显示空候选。
UPDATE iam_scope_types SET status=1 WHERE scope_code IN
  ('creator','organization','environment','crawler_project','content_channel','region') AND status=0;

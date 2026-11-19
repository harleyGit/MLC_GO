USE HG_MLC_DB;

-- 停用前按 scope_type 点查绑定，避免在亿级授权表上扫描。
ALTER TABLE iam_user_role_bindings ADD KEY idx_iam_binding_scope_type(scope_type,status,id);

CREATE TABLE iam_scope_types (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '类型主键',
  scope_code VARCHAR(32) NOT NULL COMMENT '稳定作用域编码',
  scope_name VARCHAR(64) NOT NULL COMMENT '显示名称',
  description VARCHAR(255) NOT NULL DEFAULT '' COMMENT '业务用途',
  requires_scope_id TINYINT(1) NOT NULL DEFAULT 1 COMMENT '非全局作用域必须携带已登记 ID',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1 启用、0 停用',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY(id), UNIQUE KEY uk_iam_scope_code(scope_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='IAM 作用域类型字典';

-- 仅既有 global/shop/org 可授权；其余类型待接入权威资源表与权限引擎后再启用。
INSERT INTO iam_scope_types(scope_code,scope_name,description,requires_scope_id,status) VALUES
('global','平台全局','平台级角色授权',0,1),
('shop','店铺','店铺商品、订单和成员管理',1,1),
('org','组织（旧编码）','现有 IAM 组织作用域兼容编码',1,1),
('creator','创作者空间','创作者视频和内容管理',1,0),
('organization','组织','企业或内部组织权限',1,0),
('environment','运行环境','开发、测试和生产环境运维权限',1,0),
('crawler_project','爬虫项目','爬虫任务和抓取策略',1,0),
('content_channel','内容频道','视频频道运营和审核',1,0),
('region','业务区域','区域运营和商家管理',1,0);

USE HG_MLC_DB;

-- 标记有独立权限子集的绑定；没有标记的历史绑定沿用角色全部当前权限。
CREATE TABLE iam_binding_permission_limits (
  binding_id BIGINT UNSIGNED NOT NULL COMMENT '用户角色作用域绑定主键；存在即启用权限子集，空子集表示无权限',
  PRIMARY KEY(binding_id),
  CONSTRAINT fk_iam_bpl_binding FOREIGN KEY(binding_id) REFERENCES iam_user_role_bindings(id) ON DELETE CASCADE
) ENGINE=InnoDB COMMENT='IAM 绑定权限子集标记';

CREATE TABLE iam_binding_allowed_permissions (
  binding_id BIGINT UNSIGNED NOT NULL COMMENT '已启用权限子集的用户角色作用域绑定主键',
  permission_id BIGINT UNSIGNED NOT NULL COMMENT '允许的权限主键；最终仍须属于当前角色权限集合',
  PRIMARY KEY(binding_id,permission_id),
  CONSTRAINT fk_iam_bap_limit FOREIGN KEY(binding_id) REFERENCES iam_binding_permission_limits(binding_id) ON DELETE CASCADE,
  CONSTRAINT fk_iam_bap_permission FOREIGN KEY(permission_id) REFERENCES iam_permissions(id)
) ENGINE=InnoDB COMMENT='IAM 绑定允许权限子集';

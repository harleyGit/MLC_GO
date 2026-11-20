USE HG_MLC_DB;

-- 仅可在确认 iam_binding_permission_limits 无记录且已回退所有依赖此表的服务实例后执行；
-- 删除非空限制表会使受限绑定继承角色全部权限，属于权限扩张，禁止直接用于线上回滚。
DROP TABLE IF EXISTS iam_binding_allowed_permissions;
DROP TABLE IF EXISTS iam_binding_permission_limits;

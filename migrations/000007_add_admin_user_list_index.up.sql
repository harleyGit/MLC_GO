-- ============================================================
-- 管理员列表索引优化
-- 版本: 000007
-- 描述: 为 admin_user 默认列表 cursor 分页补充软删除过滤 + 主键排序复合索引
-- ============================================================

-- 用途：支撑 GET /api/v1/ops/admins/list 的 WHERE is_delete=0 AND id<? ORDER BY id DESC LIMIT ? 查询。
-- 性能边界：避免亿级 admin_user 表在软删除过滤后做深分页/OFFSET 或额外 filesort；该迁移只建索引，不改数据。
ALTER TABLE `admin_user`
  ADD INDEX `idx_is_delete_id` (`is_delete`, `id`);

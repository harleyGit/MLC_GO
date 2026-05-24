-- ============================================================
-- 课程平台数据库迁移回滚脚本
-- 版本: 000006
-- 描述: 撤回课程平台核心业务表结构
-- 执行顺序: 按照依赖关系逆序删除
-- ============================================================

-- 删除云存储资源模块表
DROP TABLE IF EXISTS `resource_upload_files`;

-- 删除短信模块表
DROP TABLE IF EXISTS `sms_template`;

-- 删除订单模块表（先删明细表，再删主表）
DROP TABLE IF EXISTS `order_items`;
DROP TABLE IF EXISTS `orders`;

-- 删除课程商品模块表（先删关联表，再删主表）
DROP TABLE IF EXISTS `user_course_goods`;
DROP TABLE IF EXISTS `course_lessons`;
DROP TABLE IF EXISTS `course_catalog`;
DROP TABLE IF EXISTS `course_goods`;

-- 删除用户体系模块表（先删关联表，再删主表）
DROP TABLE IF EXISTS `app_user`;
DROP TABLE IF EXISTS `wechat_user`;
DROP TABLE IF EXISTS `user`;

-- 删除权限体系模块表（先删关联表，再删主表）
DROP TABLE IF EXISTS `admin_user_role`;
DROP TABLE IF EXISTS `role_permission`;
DROP TABLE IF EXISTS `admin_user`;
DROP TABLE IF EXISTS `permission`;

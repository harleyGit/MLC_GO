-- 发送记录按创建人和主键倒序游标读取；任务量增长时避免在多个 status 上排序。
-- 发布版列表按状态、主键游标定位，不依赖最新草稿版本。
ALTER TABLE notification_send_tasks ADD KEY idx_notification_send_mine (created_by,id);
ALTER TABLE notification_templates ADD KEY idx_notification_template_published (status,id);

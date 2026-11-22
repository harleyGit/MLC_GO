# 有界站内模板发送（第一阶段）

复用 `notification_templates`/`notification_template_versions`、既有用户收件箱和事件去重账本；新增 `notification_send_tasks` 持久任务。发布前由发布系统先部署 `migrations/000083_notification_send_tasks.up.sql`，再启动新版本服务。仓库内不自动执行迁移或真实发送。

## 操作

1. 使用已有 `POST /api/v1/admin/notification/templates` 创建模板草稿（字段沿用既有 `snake_case` 协议）；当前仅支持纯文本和字符串变量。运营发送仅接受 `system` 或 `marketing` 类别、`channels:["in_app"]`、`action_type:"none"`；不能填写验证码、密码、令牌等敏感正文。
2. **另一名**持有 `notification:template:publish` 权限的管理员请求 `POST /api/v1/admin/notification/templates/publish-send?id=<templateId>`，正文 `{"version":1}`。原有 `/templates/publish` 仍只处理 `iam.role.updated`。
3. 持有 `notification:send:create` 权限的管理员请求 `POST /api/v1/admin/notification/send-tasks`，携带 `Idempotency-Key`（1–128 ASCII 字符）及：

   ```json
   {
     "templateId": "7",
     "templateVersion": 1,
     "userIds": ["user_1001", "user_1002"],
     "variables": {"nickname": "用户"}
   }
   ```

4. `GET /api/v1/admin/notification/send-tasks/detail?id=<taskId>` 查询本人任务；`POST /api/v1/admin/notification/send-tasks/cancel?id=<taskId>` 取消尚未处理的用户。回执 `pending` 只表示已保存任务；`processed` 包含营销偏好拒收，不等于实际送达或阅读。

发布/创建/读取/取消使用独立 IAM 权限；迁移只向活动 `super_admin` 预置权限。接收人 ID 必须已存在，每个任务最多 100 人；同一管理员最多 20 个未完成任务。接收人按排序固定快照，任务绑定不可变版本；重用同一请求键但变更负载返回冲突。

## 边界与恢复

每 Pod 单工作协程，每秒至多处理一名接收人。MySQL `FOR UPDATE SKIP LOCKED` 短租约负责多 Pod 抢占，超时由其他工作者恢复；既有 `(event_id, receiver_id, category)` 永久去重账本保证重复租约不会产生重复入箱。失败按 2–32 秒退避，连续 5 次变为 `failed` 并保留现场，需人工审核后制定补偿；取消只阻止后续处理，不撤回已落库消息。

这是**明确用户 + 即时纯站内通知**的有界闭环，不提供群组/全站/定时、Kafka、短信、邮件、Centrifugo 实时推送或自动 DLQ 重放；上述能力须另行设计审批、分片、审计与容量压测后上线。收件箱仍使用既有单表索引，不能以此推断千万级吞吐或延迟指标。任务记录包含用户 ID 与通知变量，需按生产数据保留/删除策略控制访问和清理，不能盲目删表以免丢失幂等依据。

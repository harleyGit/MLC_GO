# 有界站内模板发送与双人任务审批

隔离环境验收步骤与 Kafka / 实时推送的待确认边界见 [`hg_rollout_plan.md`](hg_rollout_plan.md)；这份计划不表示新增链路已接通。

复用 `notification_templates`/`notification_template_versions`、既有用户收件箱和事件去重账本；新增 `notification_send_tasks` 持久任务。发布前由发布系统按序部署 `migrations/000083_notification_send_tasks.up.sql`、`migrations/000084_notification_send_read_indexes.up.sql`、`migrations/000085_notification_send_approval.up.sql` 和 `migrations/000086_notification_send_kafka_mode.up.sql`，再启动新版本服务。Kafka 专用消费组在 debug 配置中仍为关闭状态，不能把迁移完成当作 Kafka 发送已验收。

## 操作

通用模板可在创建草稿后通过 `POST /api/v1/admin/notification/templates/publish-general?id=<templateId>` 携带 `{"version":1}` 发布：要求 `notification:template:publish` 全局权限、当前保存版本和另一名复核人；只有具备有效全局 `super_admin` 绑定且该绑定具有发布权限的管理员可自审。角色更新模板 `iam.role.updated` 仍只能走原专用发布接口。通用发布将版本标记为 `published_general`，发布后可从 `/templates/published` 取得不可变快照及 `publication_status`；**不会触发投递，且不能用于运营站内发送任务**。同一模板原有站内发送版若被后续通用发布替换，新任务不再可选择旧版，须再经过 `/publish-send` 审核；已创建任务仍绑定历史发送版执行。只有未来业务显式接入且按编码查询发布版、核验分类/变量/渠道后，才会消费此快照；当前没有按编码读取的公开业务接口。此前版本的公开站内发送入口不变。

1. 使用已有 `POST /api/v1/admin/notification/templates` 创建模板草稿（字段沿用既有 `snake_case` 协议）；当前仅支持纯文本和字符串变量。运营发送仅接受 `system` 或 `marketing` 类别、`channels:["in_app"]`、`action_type:"none"`；不能填写验证码、密码、令牌等敏感正文。
2. 持有 `notification:template:publish` 权限的管理员请求 `POST /api/v1/admin/notification/templates/publish-send?id=<templateId>`，正文 `{"version":1}`。通常须由**另一名**管理员复核；持有效全局 `super_admin` 角色绑定且该绑定含发布权限的管理员可以发布本人创建的版本，服务端核对身份与绑定并将自审单独记为 `published_self` 审计动作。其他角色仍不得自审。此前通用发布的当前版本也须再次通过站内发送校验与复核，升级为 `published` 才可供发送。原有 `/templates/publish` 仍只处理 `iam.role.updated`。
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

定时发送可在创建体添加 `"scheduledAt":"2026-11-23T02:00:00.000Z"`（UTC，毫秒精度，未来 30 天内）。定时任务与营销类即时任务均先进入 `pending_approval`，不由 Worker 领取；另一名有 `notification:send:approve` 权限的管理员通过 `GET /api/v1/admin/notification/send-tasks/review` 查询待审任务，`GET /api/v1/admin/notification/send-tasks/review/detail?id=<id>` 核对服务端渲染后的文案，`POST /api/v1/admin/notification/send-tasks/approve?id=<id>` 提交 `{}` 放行。审批不修改计划时间；取消可阻止尚未投递的接收人。本人任务列表与详情可查看 `scheduledAt`；未批准的任务不投递。

`GET /api/v1/admin/notification/templates/published?cursor=<id>` 返回当前已发布版本快照（每页 20 条），`publication_status` 区分通用版 `published_general` 与专用版 `published`；创建新草稿不会替换发布内容。旧客户端即便把通用发布版误当作发送选项，服务端发送任务仍以版本状态 `published` 拒绝。`GET /api/v1/admin/notification/send-tasks?cursor=<id>` 返回本人任务倒序游标记录（每页 20 条），不包含接收人和变量。两者依赖 `migrations/000084_notification_send_read_indexes.up.sql` 的排序索引；通用发布审计依赖既有 `000085` 的 `notification_template_audit`，必须先在发布流程执行相关迁移再启动提供这些接口的新服务版本。

发布/创建/读取/取消使用独立 IAM 权限；迁移只向活动 `super_admin` 预置权限。接收人 ID 必须已存在，每个任务最多 100 人；同一管理员最多 20 个未完成任务。接收人按排序固定快照，任务绑定不可变版本；重用同一请求键但变更负载返回冲突。

## 边界与恢复

每 Pod 单工作协程，每秒至多处理一名接收人。MySQL `FOR UPDATE SKIP LOCKED` 短租约负责多 Pod 抢占，超时由其他工作者恢复；既有 `(event_id, receiver_id, category)` 永久去重账本保证重复租约不会产生重复入箱。失败按 2–32 秒退避，连续 5 次变为 `failed` 并保留现场，需人工审核后制定补偿；取消只阻止后续处理，不撤回已落库消息。运营模板发布及新任务创建、审批、取消与审计记录分别同事务提交，记录操作者、版本、服务端 context 请求 ID 与直连 IP（不信任客户端请求头或代理转发头）；模板发布审计另记录审核时快照的 SHA-256，并以 `published_self` / `published` 区分自审和双人复核。旧任务没有回填审计；后台逐人重试没有 HTTP 请求 ID，也尚未写入此审核审计表。`000085` 回退只能在所有 `pending_approval` 已处理、且明确接受删除审批审计数据时由发布流程执行。

这是**明确用户 + 可定时 + 纯站内通知**的有界闭环。Kafka 模式已具备任务级隔离、到期 Outbox 与代次消费的受控代码，但专用消费组仍关闭，尚未完成独立 topic、真实投递与故障恢复验收；不能把它称为已可用。站内信首次入箱提交后，在现有 `MLC_IM_REALTIME_ENABLED` 启用且会话绑定的本人 `im:iam:<user>` 频道可用时发送 `notification.inbox.updated.v1` 分类提示；不包含正文、接收人或已读回执。网关故障不回滚入箱，也不重试 Socket；Web 首次打开、重连后按本人 HTTP 收件箱补齐。未提供群组/全站、短信、邮件、实时必达或自动 DLQ 重放。收件箱仍使用既有单表索引，不能以此推断千万级吞吐或延迟指标。任务记录包含用户 ID 与通知变量，需按生产数据保留/删除策略控制访问和清理，不能盲目删表以免丢失幂等依据。已发布版本哈希只绑定模板，不绑定每次任务填写的变量，仍需隔离环境和发布审批验收。

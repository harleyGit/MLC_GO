# 有界通知发送：隔离验收与后续链路决策

本文件是执行前检查清单，**不授权自动迁移、连接服务或发送真实通知**。当前已实现的权威投递链路是 MySQL `notification_send_tasks` 租约工作者；每 Pod 一个工作协程，每次处理一名用户。`pending` 和 `processed` 都不是实时到达或用户已读证明。

## 隔离环境验收门槛

1. 由环境负责人明确隔离环境、数据库和 Kafka/Centrifugo 的归属、测试账号与接收账号、清理方式；不要借用共享 pre/prod，也不要使用真实用户或真实模板变量。没有这些信息，只运行不连接外部服务的单元测试。
2. 发布方先核查 `000083`、`000084` 的迁移状态和 `notification_send_tasks` 行数、索引及现有待处理任务。`000085` 包含 `ALTER TABLE` 与两张审计表，MySQL DDL 会隐式提交；先评估锁等待和发布窗口，不能将多语句脚本视为整体可回滚事务。
3. 在隔离环境按序执行迁移后核对 `scheduled_at`、`approved_by`、`approved_at`、`idx_notification_send_review`、`notification_send_audit`、`notification_template_audit` 与 `notification:send:approve` 的授权；确认已有 `pending` 任务未被改成待审。再启动新版本服务，不倒置顺序。
   只读预检入口：`go run ./cmd/hg_config_check --env debug --config-dir ./config --check migration-version` 与 `--check notification-send-schema`；结构就绪不等于功能验收。
4. 使用两个不同管理员、至多两名专用测试接收人及唯一幂等键：分别验证立即系统任务、定时系统任务、立即营销任务、自审拒绝、他人审批、审批前不投递、计划时间前不投递、营销偏好拒收、同键重放、取消与工作者重试。按任务主键核对任务状态、审计、收件箱与永久去重账本；重复投递不得产生第二条站内通知。只在隔离环境用隔离数据验证，**不在本仓库自动执行**。
5. 压测前先测量数据库连接池、`idx_notification_send_ready` 扫描、租约过期恢复、审计写入延迟和每 Pod 实际速率；没有实测不得宣称千万级容量。记录失败态、恢复办法与清理方案。`000085` 的 down 脚本会删除审计数据，且在存在 `pending_approval` 时不得运行；上线回退优先停新写入、保留数据并回滚应用，不盲目执行 down。

## Kafka / 实时接入的待确认边界

- 现有 `hg_kafka_application.go` 已运行 `outbox` dispatcher；`internal/consumer/notification` 目前仅处理已提交的关注事件。不能只向现有 topic 写 `notification.send.requested`：旧消费者会忽略它，并可能提交 offset。若新旧 MySQL 工作者与 Kafka 消费者同时执行同一个任务，即使收件箱去重，取消、失败次数和进度也会有两个竞争来源。
- **已选定的目标是 Kafka 发送，但切换前 MySQL 工作者继续是唯一权威发送者。**下一阶段应加任务级 `delivery_mode`（旧任务默认 `mysql`）与代次：MySQL 认领 SQL 只看 `mysql`，Kafka 定时扫描器只看 `kafka`；两端都以任务行原子校验模式、代次、游标和租约。新任务改用 Kafka 前，先灰度启动独立消费者并只处理专用测试任务。停用 Kafka 时禁止新建 Kafka 任务，但已有 Kafka 任务要继续消费至终态或明确补偿，不得简单改回 MySQL 形成双路径。
- 专用组的 `enabled` 与管理入口的 `new_tasks_enabled` 分离，两个开关在 debug 默认都是 `false`。灰度只开前者并通过隔离测试程序创建专用 Kafka 任务，不得把“消费者已运行”误认为“所有新管理任务可切换”；全量新任务切换须单独确认，关闭组前先核对现存 `kafka` 非终态任务。
- Kafka topic 应独立于已有关注通知消费 topic；Outbox 与任务状态在同一事务提交，每个到期游标发出不含 PII 的 `notification.send.ready.v1` 事件（`taskId`、`nextIndex`、`generation`，按 task ID 分区）。调度器发出事件时把任务状态设为已排队，过期未认领须重新发**新代次**；重复/旧代次事件不得推进新任务游标。消费成功（包括去重入箱）后再推进任务状态，数据库写入失败不得提交 offset。消息体中不复制受众或变量，收件箱仍按 `(event_id,receiver_id,category)` 的稳定业务 event ID 去重。
- 需分别测到期任务、重复 Kafka 事件、取消竞争、消费超时后再认领、Outbox 已投递但数据库回执失败、DLQ 与消费者重启；代码已有独立事件、调度器和关闭的消费组，但尚未通过真实投递/故障恢复验收。`000086` 必须先于引用 `delivery_mode` 的应用部署，且在专用 topic 创建并确认副本数/ISR 前，不得打开 Kafka 模式。`outbox_event` 仍为 pending 的代次不会因排队超时而无限重写 Outbox；若出站进入 dead，则任务保留 queued 待人工核对 Kafka 实际发布与补偿，不自动改回 MySQL，以免重复或丢失。不能把测试通过等同生产容量。
- 实时是消息中心的有损提示，不是权威投递。现有 `HGCentrifugo.HGPublishUser` 服务的是 `im:iam:<user>` 权限快照频道；通知不能复用 IAM 事件语义，也不能把正文或接收人清单广播给其他用户。建议使用仅当前认证用户可订阅的独立通知频道、独立授权与事件类型，只推通知 ID / 未读数变化提示；客户端连接和重连后按既有收件箱 API 补齐，丢失实时事件不丢消息。
- 若要求入箱后可靠地产生实时提示，需在 `HGDeliver` 的**同一 MySQL 事务**写出站事件，随后由受控消费者向 Centrifugo 发布；直接在事务外调用网关只能做到尽力而为。事件出站、订阅授权、存储保留与取消后行为会触及公共模块及安全边界，须单独确认方案与回退方式。

上述 Kafka/实时方案仍是决策草案，不表示已接通 Kafka 任务发送或 Centrifugo 通知频道。独立通知频道需要服务端鉴权、客户端权限检查与重连补偿的配套测试，不通过现有 IAM token 推断其权限安全。

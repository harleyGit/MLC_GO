# 站内通知接口与部署边界

独立站内通知不复用弹幕广播或私信 IM。通知通过 `notification.HGNewService(repo).HGDeliver(ctx,eventID,receiver,notice)` 写入 MySQL；调用方须提供稳定事件 ID，重放由 `(event_id,receiver_id,category)` 唯一键去重。新增 **关注成功** 事件闭环：关系状态变更和 `user.follow.committed` Outbox 同一事务，dispatcher 发往业务 topic；独立 `notification` 消费组根据接收人偏好入箱，落库成功后才允许提交 offset。原 `user.follow.changed` 是命令，不用于通知。订单支付、审核通过、点赞/评论等尚未接入，不能宣称所有业务事件已自动通知。

部署顺序：**先评审并执行 `migrations/000051`～`000053`，再发布 Go 与 Web；隔离环境联调后才单独启用 `kafka.business.consumers.notification`**。本机独享 debug MySQL 已按顺序执行 `000051`～`000053`，只读检查结果 `migration_version=53 dirty=false`、`notification_counter_ready=true`；pre/prod 未迁移。管理员仅在有 `notification:channel:read` / `notification:delivery:read` / `notification:channel:send` 的 IAM 授权后可访问管理端。普通角色不自动扩权。Kafka 消费至少一次，唯一索引保护同事件入箱；死信、偏好变更后的历史事件重放应单独评估。

| 路径 | 行为 |
| --- | --- |
| `GET /api/v1/notifications?category=system&cursor=&pageSize=8&unreadOnly=0` | JWT 本人收件箱，返回 `{list,nextCursor}`，游标为不透明十进制 ID；每页最多 50 条。 |
| `GET /api/v1/notifications/unread` | 本人分类未读汇总 `{unreadByCategory}`。 |
| `POST /api/v1/notifications/read` | `{notificationIds:["123"]}` 最多 50 条；或 `{category:"system",all:true}` 一次最多 500 条，返回 `{hasMore}`。 |
| `POST /api/v1/notifications/delete` | `{notificationId:"123"}` 本人软删除。 |
| `GET/PUT /api/v1/notification/preferences` | 分类偏好；关闭系统通知、启用未上线渠道或免打扰/摘要调度会拒绝保存。 |
| `GET /admin/v1/notification/channels` | 报告站内信可用，其他渠道尚未接入；无虚构成功率。 |
| `GET /admin/v1/notification/deliveries?cursor=&pageSize=20` | 仅最近已落库站内信，不是供应商投递状态。 |
| `POST /admin/v1/notification/test-send` | `{code:"in_app"}` 仅向当前管理员本人发送真实系统通知。 |

**容量与风险**：本期为单库方案，不具备千万在线、亿级历史消息的实测容量。新装空库的未读汇总读取 16 个 MySQL 计数分片；迁移时已有通知则保持 `ready=0`，继续读取原收件箱索引，禁止未完成所有用户重建就切换全局 ready。写入、已读、软删除和计数在同一事务。`all:true` 每次最多 500 条；生产需实测索引、锁争用、连接池与 P95/P99。管理员试发送不代表所有业务事件已自动触发通知。

IM **只在 `SERVER_ENV=debug` 注册 JWT 保护的 HTTP 试运行路由**，pre/prod 不注册，不能称为可用的实时单聊产品。`000052` 已在本机 debug 执行；`internal/modules/im/` 提供独立会话/成员/消息/设备待 ACK/媒体元数据结构。`GET/POST /api/v1/im/conversations` 为本人会话列表及创建，`GET/POST /api/v1/im/messages` 为 seq 补拉及文本发送，`POST /api/v1/im/read` 推进本人已读水位，`POST /api/v1/im/device/register|pending|ack` 提供单设备持久待确认列表与客户端显式 ACK。HTTP 返回已发送只表示 MySQL 消息、设备待送达记录与 IM Outbox 同事务提交；设备 30 天不活跃失活。debug Web 页从真实接口拉取并每 8 秒补拉，不是 WebSocket 即时送达。设备 secret 暂存 debug 标签页 sessionStorage，不能在跨域、共享电脑或未完成 XSS 审查时推广生产。

**仍缺失**：独立 Kafka IM 消费组和分发 worker、跨节点唤醒与可靠重试、WebSocket 网关及短期票据/Origin 校验、私有对象存储上传/审核/下载授权、离线设备重建待投递与生产压测。媒体授权器未注入时发送会拒绝，绝不使用公开 CDN URL 冒充私有链路；不得将本次 HTTP 对接称为完整 IM 上线。

## 本期增量的上线门禁

1. `000051`～`000053` 已在用户确认的本机独享 debug 执行；pre/prod 未执行。上线前必须评审数据库字符集、索引体积及回滚语义。`000052/000053` 的 down 会删除私信、通知归档及去重墓碑，生产不可执行。
2. debug HTTP 试运行暂将 `im.message.committed` Outbox 事件投递到现有业务 topic，按会话 ID 分区；生产必须先由 IaC 建立独立 IM topic、配置副本/ISR 和分发消费组。当前没有 IM 网关/消费者，不能把 Outbox 成功当作设备送达。
3. `HGRebuildUnreadOffline` 仅允许停写窗口逐用户重建计数，单用户超一万未读时拒绝继续，未提供全量在线重建。`HGArchiveBatchOffline` 仅搬运已软删除且到期的通知，单次 ID 窗口一万、最多 100 行；归档按 receiver 哈希分成 16 个物理分区并保留事件去重墓碑。未启用自动任务；在线可见通知不会移出活跃表。亿级环境还需评审墓碑容量、保留期、扫描游标和归档可恢复性，不能对大表无界回填。
4. 未提供压测目标 QPS/并发、隔离 Kafka/Redis/S3、数据生成与清理授权；本轮未做负载压测或真实 IM 链路验收，编译通过不是容量达标。不能把本机 debug MySQL 隔离等同于全链路依赖隔离。

隔离环境确认后，可用 `scripts/hg_notification_read_load.py` 对**只读** `GET /api/v1/notifications/unread` 执行显式 HTTPS 主机、最多 100 QPS/32 workers/5000 次的有界探测；从临时环境变量 `MLC_LOAD_JWT` 读取访问凭据，不在命令行打印，不跟随重定向。该脚本不写入通知，不验证亿级存储、消息端到端一致性或 IM 送达；正式压测须另列数据生成、回收、指标和资源预算。

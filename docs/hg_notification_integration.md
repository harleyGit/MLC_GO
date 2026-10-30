# 站内通知接口与部署边界

独立站内通知不复用弹幕广播或私信 IM。通知通过 `notification.HGNewService(repo).HGDeliver(ctx,eventID,receiver,notice)` 写入 MySQL；调用方须提供稳定事件 ID，重放由 `(event_id,receiver_id,category)` 唯一键去重。新增 **关注成功** 事件闭环：关系状态变更和 `user.follow.committed` Outbox 同一事务，dispatcher 发往业务 topic；独立 `notification` 消费组根据接收人偏好入箱，落库成功后才允许提交 offset。原 `user.follow.changed` 是命令，不用于通知。订单支付、审核通过、点赞/评论等尚未接入，不能宣称所有业务事件已自动通知。

部署顺序：**先评审并执行 `migrations/000051_notification_inbox.up.sql`，再发布 Go 与 Web；隔离环境联调后才单独启用 `kafka.business.consumers.notification`**。本轮不执行任何迁移或数据写入验收；无表时通知接口会返回服务暂不可用。管理员仅在有 `notification:channel:read` / `notification:delivery:read` / `notification:channel:send` 的 IAM 授权后可访问管理端。普通角色不自动扩权。Kafka 消费至少一次，唯一索引保护同事件入箱；死信、偏好变更后的历史事件重放应单独评估。

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

**容量与风险**：本期为可运行的单库闭环，不具备千万在线、亿级历史消息的已验证容量。未读汇总仍扫描单用户未读索引范围；热点用户须以可从 MySQL 重建的分片计数替换。`all:true` 会限制单次写入为 500 条，前端需要手动继续。生产需先评估分区/归档、索引大小、DB 连接池、限流与 P95/P99，使用隔离环境压测，不能凭索引推断容量。管理员试发送不代表 Kafka/Outbox 跨服务可靠送达；业务接入前需确保源事务和事件投递一致，消费者幂等并只在写入成功后提交 offset。

IM 仍**没有对外注册 HTTP/WebSocket 路由**，不能称为可用的单聊产品。`migrations/000052_im_single_chat.up.sql`（未执行）及 `internal/modules/im/` 实现了独立 MySQL 会话/成员/消息/设备待 ACK/媒体元数据结构，内部服务支持单聊创建、已认证用户发送文本、按会话 seq 拉历史、注册最多五台设备和分别 ACK；发送同事务写入消息、按活跃设备生成待送达记录与 IM Outbox。设备 30 天不活跃失活，撤销后历史仍可按成员鉴权补拉。**缺失**：服务端设备凭据安全存储客户端适配、独立 Kafka IM 消费组和分发 worker、跨节点唤醒与可靠重试、WebSocket 网关、浏览器短期票据和 Origin 校验、私有对象存储上传/审核/下载授权、离线设备重建待投递、限流、归档与生产压测。媒体授权器未注入时发送会拒绝，绝不使用公开 CDN URL 冒充私有链路。必须先补齐并在独享隔离环境验收，再开放任何 IM 路由。

## 本期增量的上线门禁

1. `000051` 和 `000052` 都只编写未执行；必须评审数据库字符集、索引体积及回滚语义。`000052` 的 down 会删除私信数据，生产不可执行。
2. IM topic 必须由 IaC 预建并配置分区、副本与 ISR，Outbox dispatcher 才可按会话 key 发布；当前没有网关/消费者，不能把 Outbox 成功当作设备送达。
3. 通知未读仍扫描单用户未读索引；可重建分片计数与归档分片尚未上线。对亿级历史表的分区/归档需要先确定历史保留、去重墓碑、双写切换与分片路由，不能对在线大表执行无界回填。
4. 没有提供独享隔离环境的配置来源、资源上限与清理授权，本轮不执行迁移、负载压测或真实依赖验收；编译通过不是容量达标。

隔离环境确认后，可用 `scripts/hg_notification_read_load.py` 对**只读** `GET /api/v1/notifications/unread` 执行显式 HTTPS 主机、最多 100 QPS/32 workers/5000 次的有界探测；从临时环境变量 `MLC_LOAD_JWT` 读取访问凭据，不在命令行打印，不跟随重定向。该脚本不写入通知，不验证亿级存储、消息端到端一致性或 IM 送达；正式压测须另列数据生成、回收、指标和资源预算。

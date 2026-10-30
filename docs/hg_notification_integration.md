# 站内通知接口与部署边界

本期交付独立站内通知，不是弹幕广播或私信 IM。通知通过 `notification.HGNewService(repo).HGDeliver(ctx,eventID,receiver,notice)` 写入 MySQL；业务调用方须提供稳定的事件 ID，重放由 `(event_id, receiver_id, category)` 唯一键去重。当前仅管理员本人试发送接入真实生产者；订单/审核/互动事件尚未接入 Outbox 消费链，不能声称业务事件自动通知。

部署顺序：**先评审并执行 `migrations/000051_notification_inbox.up.sql`，再发布 Go 与 Web**。本轮只编写迁移，不执行；无表时通知接口会返回服务暂不可用。管理员仅在有 `notification:channel:read` / `notification:delivery:read` / `notification:channel:send` 的 IAM 授权后可访问管理端。普通角色不自动扩权。

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

IM 协议扩展位于 `internal/modules/im/hg_protocol.go`，仅定义可信 sender、clientMsgId、会话 seq、私有 mediaId 授权接口。**没有 IM HTTP/WebSocket 路由，也没有已实现的文字/图片/视频发送**；禁止将弹幕 Pub/Sub 当作可靠 IM 消息队列。未来接入需另行设计会话分片、独立消息表、Outbox、Kafka 分发、私有对象存储与审核、断线补拉和容量验证。

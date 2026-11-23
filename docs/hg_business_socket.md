# 用户业务频道与 Chat 设备频道

- 已有 Centrifugo `im:iam:<user>` 是**用户私有业务频道**；保留旧名称和已有 connect proxy 协议以兼容当前客户端及滚动发布。`transport.HGBusinessChannel` 与旧 `HGAccessChannel` 同值；`HGPublishUser` 的用户 ID 来自服务端权威业务身份，不接受浏览器指定目标频道。
- `/api/v1/iam/me/realtime` 使用 JWT 的用户及 sid 签发用户会话票据。网关私有 connect proxy 复核 Redis 登录会话和用户未删除状态，自动订阅本人业务频道；登出按 sid 精确断开。IAM 快照和站内通知提示已发布到该频道；通知提示仅含分类，不含正文。新增业务事件应使用版本化 `event_type`、限定枚举及最小载荷，并明确发送方身份、对象归属与断线后的权威补拉入口。
- `im:<device>` 仍是 Chat 私信设备频道：凭设备凭据签发独立连接 token，RPC 按用户与设备再次校验。不得将设备频道追加到用户业务会话，或在业务频道上放宽私信 RPC 鉴权；连接 token、设备 secret 不进入 URL、广播载荷与日志。
- 两类连接分别受网关心跳与应用生命周期管理；网关受理发布不等于通知已读或消息设备 ACK。单标签业务 Socket 复用只减少客户端重复连接，不等于跨标签共享连接，也不构成高并发容量证明。

回归优先级：用户/设备频道隔离、跨账号与会话撤销拒绝、同连接多事件正确分发、断线补拉、未知类型忽略。不可擅自运行真实 Kafka、Redis、MySQL 集成验收或迁移。

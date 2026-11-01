选择：
- 完全自建、希望保持 Apache 2.0：SeaweedFS。
- 已经有大型存储运维团队：Ceph。
- 不想维护存储基础设施：S3。
特别注意：SeaweedFS 和 Ceph 是存储系统，不是图片处理系统。图片压缩、缩略图、视频转码仍需要单独的媒体处理服务。
2. 图片上传的正确流程
推荐采用预签名 URL（Presigned URL）直传对象存储。

```go
React
  |
  | 1. 请求上传凭证
  v
Go Media Service
  |
  | 2. 校验 JWT、文件类型、大小、权限
  | 3. 创建上传记录
  | 4. 生成 Presigned URL
  v
React
  |
  | 5. PUT 图片二进制数据
  v
SeaweedFS / S3
  |
  | 6. 上传完成
  v
Go Media Service
  |
  | 7. 校验文件、计算 Hash、生成元数据
  | 8. 返回 file_id
  v
React
  |
  | 9. 发送 IM 消息，携带 file_id
  v
Go IM Service
```

为什么这样设计？

假设一张图片 8MB：
- 错误方式：图片经过 Go API，再经过 WebSocket，增加服务端带宽和内存压力。
- 正确方式：React 直接上传对象存储，Go 只负责鉴权和元数据。
后续图片消息只需要发送：

```json
{
  "messageType": "image",
  "fileId": "file_202610300001",
  "width": 1920,
  "height": 1080
}
```


数据库保存文件 ID、对象 Key、大小、MIME、Hash、存储位置等元数据，不保存图片本身。

对于视频、语音、附件，使用同样的设计。

四、真正的 IM 系统应该怎么设计？
这里是最重要的部分。

我建议你参考字节、B站这类产品的业务边界，而不是直接寻找一个所谓的 IM 大而全框架。

你应该自主实现 Go IM Service，分成以下模块。
1. IM 业务模块

```txt
模块	职责	核心数据
User	用户信息	用户 ID、资料
Friend	好友关系	好友、黑名单
Conversation	会话管理	私聊、群聊会话
Group	群组管理	群成员、群主、管理员
Message	消息管理	文本、图片、视频
Message Delivery	消息投递	投递状态、失败重试
Message Read	已读回执	已读位置、未读数
Message Recall	撤回管理	撤回记录
Notification	通知管理	系统通知、业务通知
Media	媒体管理	图片、视频、附件
Push	离线推送	APNs、FCM、Web Push
Call	通话管理	通话记录、房间、参与者
```

2. 消息类型设计
建议不要为图片、视频、文本分别设计完全独立的消息系统。

统一采用消息信封（Message Envelope）。

例如：

```json
{
  "messageId": "msg_100001",
  "conversationId": "conv_10001",
  "senderId": "user_100",
  "messageType": "image",
  "payload": {
    "fileId": "file_10001",
    "width": 1920,
    "height": 1080
  },
  "createdAt": 1793368800000
}
```


消息类型可以定义：
const (
    MessageTypeText  = "text"
    MessageTypeImage = "image"
    MessageTypeVideo = "video"
    MessageTypeAudio = "audio"
    MessageTypeFile  = "file"
    MessageTypeSystem = "system"
    MessageTypeCustom = "custom"
)


其中 payload 根据消息类型使用不同的 DTO 校验，不能让客户端任意传入不受限制的 JSON。
3. 核心消息链路

```txt
                 用户 A
                   |
                   | WebSocket
                   v
             Centrifugo
                   |
                   | 订阅 / 接收实时事件
                   |
                   v
              用户 B
```

```txt
用户 A
  |
  | HTTP POST /im/messages
  v
Go API Gateway
  |
  v
Go IM Message Service
  |
  +---- JWT / 权限验证
  |
  +---- 好友 / 群成员验证
  |
  +---- 消息幂等校验
  |
  +---- MySQL 持久化
  |
  +---- Outbox 事件写入
  |
  v
Kafka
  |
  +---- Message Consumer
  |       |
  |       +---- Centrifugo Publish
  |       |
  |       +---- 离线推送
  |       |
  |       +---- 未读数更新
  |
  +---- Notification Consumer
  |
  +---- Search / Analytics Consumer
```


这里有一个关键设计：

先持久化消息，再异步投递消息。

不能先通过 Centrifugo 推送成功，再尝试写 MySQL。

否则可能出现：

用户 B 收到了消息，但数据库没有保存成功。

推荐使用 Transactional Outbox：

```txt
MySQL Transaction
   |
   +---- INSERT im_message
   |
   +---- INSERT im_outbox
   |
   +---- COMMIT
            |
            v
      Outbox Publisher
            |
            v
          Kafka
            |
            v
        Consumer
            |
            v
        Centrifugo
            |
            v
        用户 B
```


这样能解决 MySQL 与 Kafka 之间的双写一致性问题。消费者仍然需要幂等处理。

这和你现有 Go 工程中的 Inbox 去重、Kafka Consumer、事件持久化设计可以使用相同的工程思想。
五、千万并发、亿级数据，数据库如何选择？
这里要把两个概念分开：
- 千万并发：主要考验连接管理、网络带宽、推送吞吐量。
- 亿级数据：主要考验数据库分片、索引、查询、归档和存储容量。
不能指望一个数据库同时解决所有问题。
1. 推荐技术组合
技术	用途	是否推荐
MySQL 8	消息、会话、好友、群组	必须
Redis	在线状态、缓存、未读数	必须
Kafka	消息事件、异步投递	强烈推荐
ClickHouse	消息统计、运营分析	推荐
OpenSearch	消息搜索	按需
SeaweedFS	图片、视频、文件	推荐

2. MySQL 消息表设计
消息表不要把所有业务数据塞在一张表里。

可以先设计：
im_message
    |
    +---- message_id
    +---- conversation_id
    +---- sender_id
    +---- client_message_id
    +---- message_type
    +---- payload
    +---- sequence
    +---- created_at
    +---- status


几个重要字段：
字段	作用
message_id	全局消息 ID
conversation_id	会话 ID
client_message_id	客户端幂等 ID
sequence	会话内递增消息序号
message_type	消息类型
payload	消息内容
created_at	消息时间

为什么需要 sequence？
例如用户 A 和用户 B 的私聊：
sequence	message_id	sender	内容
1	msg001	A	你好
2	msg002	B	你好
3	msg003	A	在吗
4	msg004	B	在的

前端通过 sequence 拉取历史消息：
GET /im/messages?conversationId=conv001&beforeSequence=100&limit=20


使用游标分页，不要使用大 OFFSET。
什么时候分库分表？
建议：
conversation_id
      |
      v
  Hash 分片
      |
      +---- MySQL Shard 00
      |
      +---- MySQL Shard 01
      |
      +---- MySQL Shard 02
      |
      +---- MySQL Shard 03
      |
      +---- ...


建议按照 conversation_id 进行消息数据分片。

这样同一会话的消息尽可能落在同一个逻辑分片，方便保证会话内消息顺序。

需要注意：
- 使用会话级序号分配机制。
- 避免每条消息都更新同一个会话热点行。
- 大群聊需要考虑写入热点。
- 每个分片独立索引、备份和归档。
- 消息 ID 不应该依赖数据库自增 ID。
不要在项目初期就部署几十个 MySQL 分片。先建立分片路由抽象，达到容量和吞吐量瓶颈后再拆分。
3. Redis 存什么？
Redis 不应该作为永久消息数据库。

可以存储：
im:user:online:{userId}
im:conversation:unread:{userId}:{conversationId}
im:conversation:last:{conversationId}
im:group:members:{groupId}
im:presence:{userId}


注意：
- 在线状态需要 TTL 和心跳续期。
- 未读数应考虑数据库权威值与 Redis 投影的一致性。
- Redis 故障后能够从持久化数据恢复。
- 不要把几千万用户的所有历史消息全部塞入 Redis。

六、Centrifugo 在千万并发架构中怎么部署？
这是整个架构的实时连接层。

Centrifugo 支持多节点部署，使用 Redis、Redis Cluster、NATS 等机制进行节点之间的消息分发。

Centrifugo+1

1. 多节点部署
                 React / Web / App
                         |
                         v
                  SLB / Load Balancer
                         |
             +-----------+-----------+
             |           |           |
             v           v           v
         Centrifugo  Centrifugo  Centrifugo
           Node 01    Node 02     Node 03
             |           |           |
             +-----------+-----------+
                         |
                         v
                  Redis / NATS
                         |
                         v
                    Go IM Service


例如：
集群	节点	目标
Centrifugo	多节点	WebSocket 连接
Redis	分片集群	Presence、实时消息分发
Go IM	多实例	消息业务
Kafka	多 Broker	异步事件
MySQL	多分片	消息持久化

Centrifugo 官方 FAQ 建议单节点通常不要超过 5～10 万连接，实际取决于硬件、消息频率、订阅分布和功能配置。官方也展示过百万连接的测试案例，但不能将测试数字直接当作生产容量承诺。

GitHub+1

因此，千万连接需要进行节点压测、分片规划和容量冗余设计。
2. Centrifugo 的 Channel 如何设计？
这里非常重要。

不要把所有用户都订阅到同一个频道。

推荐：
user:10001
user:10002
user:10003

conversation:10001
conversation:10002

group:10001
group:10002

system:10001
system:10002


解释：
Channel	用途
user:	用户私信、个人通知
conversation:	私聊会话消息
group:	群聊消息
system:	系统通知

例如：

A 给 B 发消息：
A
 |
 v
Go IM Service
 |
 v
MySQL 保存消息
 |
 v
Kafka
 |
 v
Centrifugo Publish
 |
 v
user:10002
 |
 v
B 收到消息


如果 B 当前正在查看这个会话，还可以通过：
conversation:10001


接收会话实时消息。

注意：频道名称不是权限控制。必须使用服务端签发的 JWT、订阅授权或服务端代理订阅，禁止客户端任意订阅其他用户的频道。群成员变更时，还需要处理已有连接的订阅权限撤销。

另外，不要让 Centrifugo 的短期频道历史替代 MySQL 消息历史。Centrifugo 主要是实时投递设施。

七、LiveKit 如何部署？
LiveKit 的扩展方式与 Centrifugo 不完全相同。

它是 SFU，主要消耗 CPU、网络带宽和媒体转发资源。
             Go Call Service
                    |
                    | 创建房间、签发 Token
                    v
                 LiveKit
                    |
          +---------+---------+
          |         |         |
          v         v         v
       Room A    Room B    Room C
          |         |         |
       用户 A/B   用户 C/D   用户 E/F


LiveKit 支持 Redis 共享状态与消息通信，可以部署多个节点。但同一个 Room 在当前自建架构中由一个 SFU 节点承载，因此不能简单理解为一个房间可以无限横向扩展。

LiveKit Documentation+1

部署时还需要：
- 公网 IP。
- UDP 端口规划。
- TLS 证书。
- TURN 服务。
- Redis。
- 10Gbps 或更高网络条件（根据实际媒体负载规划）。
- Prometheus 监控。
官方部署文档也特别强调了公网地址、UDP、TURN 和带宽要求。

LiveKit Documentation

我的建议：聊天系统和音视频系统分别扩容，不要放在同一套服务集群中。

八、通知系统应该怎么设计？
你的系统中至少存在三种通知，不建议混在一个模块里。
类型	示例	传输方式
IM 消息通知	收到好友私信	Centrifugo
业务通知	充值成功、订单发货、审核结果	Centrifugo + APNs/FCM
系统通知	平台公告、活动、运营消息	Centrifugo + APNs/FCM/Web Push

建议设计独立的 Notification Service。
                  Go Business Services
                          |
                          v
                     Kafka Events
                          |
                          v
                  Notification Service
                          |
             +------------+------------+
             |            |            |
             v            v            v
         Centrifugo     APNs/FCM     Web Push
             |            |            |
             v            v            v
          在线用户      iOS/Android   浏览器


这里推荐：
- iOS：Apple Push Notification service（APNs）。
- Android：Firebase Cloud Messaging（FCM）。
- Web：Web Push API。
Centrifugo 负责在线实时推送，APNs、FCM 和 Web Push 负责离线或后台通知。

不要将 Centrifugo 当成 APNs/FCM 的替代品。

通知数据库可以设计：
notification
notification_template
notification_channel
notification_delivery
notification_user_read
device_push_token


其中：
- notification_template：通知模板。
- notification_channel：推送渠道。
- notification_delivery：投递记录。
- notification_user_read：用户已读状态。
- device_push_token：设备推送 Token。
这样可以实现你之前想要的短信模板、通知模板、消息中心统一管理。

九、最终推荐的技术栈
这是我建议你在 Go 工程中采用的组合。
IM 技术栈选型清单
推荐方案
层级	技术	负责内容
Web	React	聊天 UI
Web SDK	centrifuge-js	实时连接
WebRTC SDK	livekit-client	视频通话
API	Go + net/http	业务接口
实时服务	Centrifugo	WebSocket
音视频	LiveKit	SFU
消息队列	Kafka	异步事件
消息数据库	MySQL 8	持久化
缓存	Redis	在线、缓存
对象存储	SeaweedFS	媒体文件
搜索	OpenSearch	消息检索
分析	ClickHouse	消息统计
监控	Prometheus	指标
可视化	Grafana	监控面板
部署	Kubernetes	服务编排


相关项目地址：
项目	官方仓库
Centrifugo	GitHub
LiveKit	GitHub
SeaweedFS	GitHub
Kafka	GitHub
Redis	GitHub
ClickHouse	GitHub
OpenSearch	GitHub

以上核心项目均有开源版本，但具体许可证、企业功能和版本维护状态应在正式引入前逐一审核。开源免费也不代表部署、带宽、硬件和运维没有成本。
十、Go 工程目录应该怎么组织？
结合你现在的 Go 工程风格，我建议将 IM 设计成独立业务域，而不是把 Centrifugo 的代码直接混入业务逻辑。
MLC_GO/
│
├── cmd/
│   ├── api/
│   ├── im-worker/
│   ├── notification-worker/
│   └── outbox-publisher/
│
├── internal/
│   │
│   ├── im/
│   │   ├── user/
│   │   ├── friend/
│   │   ├── conversation/
│   │   ├── group/
│   │   ├── message/
│   │   ├── delivery/
│   │   ├── receipt/
│   │   └── recall/
│   │
│   ├── realtime/
│   │   ├── centrifugo/
│   │   ├── channel/
│   │   ├── publisher/
│   │   └── auth/
│   │
│   ├── media/
│   │   ├── upload/
│   │   ├── storage/
│   │   ├── image/
│   │   └── video/
│   │
│   ├── notification/
│   │   ├── template/
│   │   ├── channel/
│   │   ├── delivery/
│   │   └── push/
│   │
│   ├── call/
│   │   ├── room/
│   │   ├── token/
│   │   └── record/
│   │
│   ├── consumer/
│   └── infrastructure/
│       ├── mysql/
│       ├── redis/
│       ├── kafka/
│       ├── clickhouse/
│       └── storage/
│
└── migrations/
    ├── im/
    ├── media/
    └── notification/


十一、实际落地应该分几个阶段？
不建议一开始就搭建千万连接集群。先实现完整业务闭环，再逐步扩容。
1. 第一阶段：基础 IM
   - Centrifugo 单节点。
   - React 接入 centrifuge-js。
   - Go 实现用户、会话、好友、群组。
   - MySQL 消息持久化。
   - 私聊、群聊、消息历史。
   - 消息幂等、离线消息拉取。
2. 第二阶段：多媒体和通知
   - SeaweedFS / S3 上传。
   - 图片、视频、语音、附件。
   - 消息撤回、已读回执。
   - Notification Service。
   - APNs、FCM、Web Push。
3. 第三阶段：异步架构
   - Kafka。
   - Transactional Outbox。
   - Inbox 幂等。
   - 消息投递重试。
   - 消息顺序保证。
   - 未读数与消息状态对账。
4. 第四阶段：实时音视频
   - LiveKit。
   - 一对一语音、视频。
   - 多人会议。
   - 房间管理。
   - 通话记录、录制。
5. 第五阶段：大规模扩容
   - Centrifugo 多节点。
   - Redis Cluster / NATS。
   - MySQL 分库分表。
   - Kafka 分区规划。
   - 对象存储集群。
   - Kubernetes 自动扩缩容。
   - Prometheus + Grafana。
   - 全链路压测、故障演练。
十二、关于千万并发，我特别强调三个问题
1. 千万在线连接不等于千万消息并发。

假设有 1000 万在线用户，但每秒只有 10 万条消息，和每秒 1000 万条消息是完全不同的系统规模。

容量规划必须明确：
- 在线连接数。
- 每秒消息写入量。
- 每秒消息推送量。
- 平均消息大小。
- 群聊人数分布。
- 消息峰值。
- 媒体带宽。
2. 大群聊需要单独设计。

例如一个拥有 100 万成员的群：

不能简单地把一条消息同步写入 100 万条用户收件箱记录。

需要采用：
- 群消息单次持久化。
- 群成员分批拉取或分批推送。
- 热门群广播分片。
- 未读数异步聚合。
- 在线与离线用户分流。
3. 必须建立消息可靠性体系。

需要解决：
- 重复投递。
- 消息乱序。
- 消息丢失。
- 客户端断线重连。
- 离线消息补拉。
- 消息状态对账。
- 消费者故障恢复。
尤其是消息的可靠性，不能只依赖 WebSocket 或 Kafka，需要业务数据库、Outbox、消费者幂等和客户端 ACK 配合。

十三、我的最终建议
如果是我来设计这套系统，我会确定以下技术边界：
业务	最终选型
即时通讯实时连接	Centrifugo
IM 业务逻辑	Go 自研
消息持久化	MySQL
异步消息投递	Kafka
在线状态	Redis
音视频通话	LiveKit
图片、视频存储	SeaweedFS / S3
搜索	OpenSearch
统计分析	ClickHouse

不要选择一个大而全的 IM 框架来控制所有业务。

你现在的 Go 工程已经有 Kafka、Redis、MySQL、ClickHouse、事件幂等、Outbox/Inbox 等基础设施设计思路。继续沿用这些架构原则，自研 IM 业务层，接入成熟的实时通信和媒体基础设施，是更合理的路线。

下一步最值得做的是把 IM 业务真正落到数据库、接口和事件协议上。

我建议下一步直接设计一份：

《Go + React 千万级 IM 即时通讯系统完整生产级技术设计方案 v1.0》

包含完整 MySQL DDL、消息协议、REST API、Centrifugo Channel/JWT、Kafka Topic、Outbox/Inbox、私聊群聊、消息可靠投递、图片上传、LiveKit 通话、部署拓扑和压测指标，做到可以直接进入编码阶段。


<br/>

***
<br/><br/><br/>
># <h1 id=""></h1>

Go + Centrifugo + LiveKit + SeaweedFS 即时通信系统技术设计方案
建议将这套技术栈设计成一个独立的 IM 即时通信与实时音视频系统，由 Go 统一负责业务、权限、数据持久化和服务编排。

技术选型：
技术	核心职责	定位
Go	用户、会话、消息、群组、权限、业务接口	业务核心
Centrifugo	WebSocket、实时消息推送、在线状态	实时通信网关
LiveKit	一对一音视频、多人语音、视频会议	实时音视频服务
SeaweedFS	图片、视频、语音、文件存储	分布式对象存储
MySQL 8	用户、会话、消息、关系、文件元数据	业务主数据库
Redis	在线状态、缓存、分布式锁、短期状态	高速缓存
Kafka	消息事件、异步处理、业务解耦	事件总线
React	聊天窗口、消息列表、音视频界面	Web 前端

核心设计原则：
1. Go 是唯一的业务权威入口，Centrifugo 不负责业务消息的最终存储。
2. 消息先持久化，再通过 Centrifugo 推送，保证离线消息可恢复。
3. LiveKit 负责音视频媒体传输，Go 负责房间、成员、权限和 Token。
4. SeaweedFS 只存储文件，MySQL 保存文件元数据和业务关联。
5. 所有消息必须支持幂等、消息游标、离线补偿和多端同步。
整体采用业务服务与实时基础设施分离的架构。
一、整体系统架构
1.1 推荐生产架构
                         React Web
                            |
             +--------------+--------------+
             |              |              |
          HTTPS           WebSocket       WebRTC
             |              |              |
             v              v              v
       +-----------+  +-------------+  +-----------+
       | Go API    |  | Centrifugo  |  | LiveKit   |
       | Gateway   |  |             |  | Server    |
       +-----------+  +-------------+  +-----------+
             |              ^              ^
             |              |              |
             +--------------+              |
             |                             |
             v                             |
       +------------------+                |
       | Go IM Services   |----------------+
       |                  |
       | User Service     |
       | Conversation     |
       | Message Service  |
       | Group Service    |
       | Media Service    |
       | Call Service     |
       | Notification     |
       +------------------+
             |
       +-----+----------------------+
       |             |              |
       v             v              v
    MySQL 8        Redis          Kafka
  业务持久化       状态缓存        事件总线
       |
       v
   SeaweedFS
 图片/视频/语音/文件


这里有三个重要边界：
- Go：业务控制平面，决定谁能发消息、谁能加入会话、消息如何保存。
- Centrifugo：实时数据传输平面，负责将 Go 产生的消息事件推送给在线用户。
- LiveKit：音视频媒体平面，负责音视频流传输，不参与普通聊天消息的持久化。
注意：Centrifugo 自身具有消息历史和恢复机制，但它的 History 是有容量和时间限制的缓存，不能代替 MySQL 的消息持久化。

Centrifugo+2

1.2 系统模块划分
建议采用模块化单体起步，内部按领域拆分，未来可以独立部署为微服务。
模块	Go 包	职责
用户模块	user	用户资料、用户关系
会话模块	conversation	单聊、群聊、会话列表
消息模块	message	文本、图片、视频、文件、撤回
成员模块	member	群成员、角色、禁言
实时模块	realtime	Centrifugo Token、消息推送
媒体模块	media	上传、下载、文件元数据
音视频模块	call	呼叫、接听、挂断、房间管理
通知模块	notification	系统通知、消息提醒
在线模块	presence	在线状态、最后活跃时间
事件模块	event	Outbox、Kafka、事件消费

建议目录：
internal/
├── user/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   └── model/
│
├── im/
│   ├── conversation/
│   ├── message/
│   ├── member/
│   ├── reaction/
│   ├── readreceipt/
│   └── recall/
│
├── realtime/
│   ├── centrifugo/
│   │   ├── client.go
│   │   ├── token.go
│   │   ├── publisher.go
│   │   └── channel.go
│   └── presence/
│
├── media/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   └── seaweedfs/
│
├── call/
│   ├── handler/
│   ├── service/
│   ├── livekit/
│   └── model/
│
├── notification/
│
└── event/
    ├── outbox/
    ├── producer/
    └── consumer/



二、即时通信核心业务设计
建议先实现以下功能。
功能	一期	二期
一对一聊天	必须	
群聊	必须	
文本消息	必须	
图片、视频、文件	必须	
语音消息	必须	
消息历史分页	必须	
离线消息	必须	
多端同步	必须	
消息已读、未读	必须	
消息撤回、删除		扩展
消息回复、引用		扩展
表情、点赞		扩展
一对一音视频		扩展
多人音视频会议		扩展
屏幕共享		扩展

2.1 会话模型
会话分为两种：
Conversation
    |
    +---- SINGLE     单聊
    |
    +---- GROUP      群聊


例如：
用户 A                     用户 B
  |                          |
  +-------- 单聊会话 --------+
               |
         conversation_id
               |
          消息记录


群聊：
              群聊会话
                  |
          conversation_id
                  |
       +----------+----------+
       |          |          |
      用户A      用户B      用户C


注意一个重要设计：

会话与会话成员必须分开存储。

一个会话可以有多个成员，每个成员有独立的未读数、最后已读消息位置、免打扰状态。
2.2 MySQL 核心表结构
以下使用 MySQL 8，业务 ID 使用 UUID 字符串，数据库自增主键作为内部索引 ID。
① 会话表 im_conversation
CREATE TABLE im_conversation (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    conversation_id VARCHAR(64) NOT NULL,

    conversation_type TINYINT NOT NULL COMMENT '1单聊 2群聊',

    title VARCHAR(128) NOT NULL DEFAULT '',

    owner_user_id VARCHAR(64) NOT NULL DEFAULT '',

    last_message_id VARCHAR(64) NOT NULL DEFAULT '',
    last_message_at DATETIME(3) DEFAULT NULL,

    status TINYINT NOT NULL DEFAULT 1,

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),
    UNIQUE KEY uk_conversation_id (conversation_id),
    KEY idx_last_message (last_message_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


② 会话成员表 im_conversation_member
CREATE TABLE im_conversation_member (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    conversation_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,

    member_role TINYINT NOT NULL DEFAULT 1
        COMMENT '1普通成员 2管理员 3群主',

    last_read_message_id VARCHAR(64) NOT NULL DEFAULT '',
    last_read_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,

    unread_count BIGINT UNSIGNED NOT NULL DEFAULT 0,

    is_muted TINYINT NOT NULL DEFAULT 0,
    is_pinned TINYINT NOT NULL DEFAULT 0,

    joined_at DATETIME(3) NOT NULL,
    left_at DATETIME(3) DEFAULT NULL,

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),

    UNIQUE KEY uk_conversation_user (
        conversation_id,
        user_id
    ),

    KEY idx_user_conversation (
        user_id,
        left_at,
        id
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


③ 消息表 im_message
这是整个 IM 系统最重要的业务表。
CREATE TABLE im_message (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    message_id VARCHAR(64) NOT NULL,

    conversation_id VARCHAR(64) NOT NULL,

    sender_id VARCHAR(64) NOT NULL,

    client_message_id VARCHAR(64) NOT NULL,

    message_seq BIGINT UNSIGNED NOT NULL,

    message_type TINYINT NOT NULL
        COMMENT '1文本 2图片 3视频 4语音 5文件 6系统',

    content JSON NOT NULL,

    reply_message_id VARCHAR(64) NOT NULL DEFAULT '',

    status TINYINT NOT NULL DEFAULT 1
        COMMENT '1正常 2撤回',

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),

    UNIQUE KEY uk_message_id (message_id),

    UNIQUE KEY uk_conversation_seq (
        conversation_id,
        message_seq
    ),

    UNIQUE KEY uk_sender_client_message (
        sender_id,
        client_message_id
    ),

    KEY idx_conversation_created (
        conversation_id,
        created_at,
        id
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


这里有三个关键字段：
字段	作用
message_id	服务端生成的消息唯一 ID
client_message_id	React 客户端生成，防止重复发送
message_seq	会话内递增消息序号，支持排序、已读和同步

例如：
conversation_id = C100

message_seq    sender    content
---------------------------------
1              A         你好
2              B         你好
3              A         在吗？
4              B         在的
5              A         图片


message_seq 是会话内的业务序号，不是数据库自增 ID。

不要使用时间戳作为消息排序的唯一依据。
④ 文件元数据表 im_attachment
CREATE TABLE im_attachment (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    attachment_id VARCHAR(64) NOT NULL,

    uploader_id VARCHAR(64) NOT NULL,

    storage_type TINYINT NOT NULL DEFAULT 1
        COMMENT '1 SeaweedFS',

    object_key VARCHAR(512) NOT NULL,

    original_name VARCHAR(255) NOT NULL DEFAULT '',

    mime_type VARCHAR(128) NOT NULL DEFAULT '',

    file_size BIGINT UNSIGNED NOT NULL DEFAULT 0,

    file_hash CHAR(64) NOT NULL DEFAULT '',

    width INT UNSIGNED NOT NULL DEFAULT 0,
    height INT UNSIGNED NOT NULL DEFAULT 0,

    duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0,

    status TINYINT NOT NULL DEFAULT 1
        COMMENT '1正常 2处理中 3删除',

    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),

    PRIMARY KEY (id),

    UNIQUE KEY uk_attachment_id (attachment_id),

    KEY idx_uploader (uploader_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


图片消息的 JSON 示例：
{
  "text": "这是一张图片",
  "attachments": [
    {
      "attachmentId": "ATT10001",
      "mimeType": "image/jpeg",
      "width": 1920,
      "height": 1080
    }
  ]
}


注意：消息表不存图片二进制数据，也不建议将 SeaweedFS 的文件内容直接放进 JSON。

消息 JSON 只保存附件业务 ID，通过附件表查询文件元数据和访问地址。
2.3 其他必要数据表
生产系统还需要：
表名	用途
im_group	群组信息
im_group_member	群成员及角色
im_message_read	已读回执明细（按需）
im_message_reaction	表情及点赞
im_message_edit	消息编辑历史
im_message_recall	消息撤回记录
im_user_block	用户屏蔽关系
im_user_device	用户设备及多端信息
im_message_outbox	消息事务发件箱
im_message_inbox	消费端事件幂等记录
im_call_session	音视频呼叫记录
im_call_participant	音视频参与者记录

其中 im_message_read 不建议为每条消息、每个群成员都生成一条记录。大群优先使用成员的 last_read_seq 表示已读进度，只有确实需要精确的逐条已读明细时才额外记录。

三、Centrifugo 实时消息架构
3.1 Centrifugo 的定位
Centrifugo 不负责业务消息创建，而是负责消息分发。
React
  |
  | HTTP POST /im/messages
  v
Go Message Service
  |
  | 1. 校验用户权限
  | 2. 校验会话成员
  | 3. 消息幂等
  | 4. 分配 message_seq
  | 5. 写入 MySQL
  | 6. 写入 Outbox
  |
  v
MySQL
  |
  | Outbox Worker
  v
Kafka
  |
  | 消息事件消费
  v
Go Realtime Worker
  |
  | Centrifugo HTTP API
  v
Centrifugo
  |
  +----------+----------+
  |          |          |
  v          v          v
 React A   React B   React C


这样设计的原因：
- 消息已经写入数据库，即使推送失败也不会丢失。
- 用户断线重连后，可以从 MySQL 获取消息。
- Kafka 消费失败可以重试。
- Centrifugo 重启不影响消息持久化。
Centrifugo 官方也推荐由后端完成业务验证和持久化后再进行发布。

Centrifugo+1

3.2 Channel 命名规范
建议统一管理 Channel，不允许 React 随意构造有权限风险的频道。
Channel	用途
user:{userID}	用户个人事件
conversation:{conversationID}	单聊、群聊消息
call:{callID}	音视频呼叫信令
system:{userID}	系统通知

例如：
user:U10001

conversation:C10001

conversation:C10002

call:CALL10001

system:U10001


具体权限：
- 用户频道：只有对应用户可以订阅。
- 会话频道：只有当前有效会话成员可以订阅。
- 呼叫频道：只有该呼叫的参与者可以订阅。
- 系统频道：由 Go 服务端发布。
建议 Centrifugo 客户端禁用普通业务 Channel 的直接发布权限，消息必须通过 Go API 提交。
3.3 Centrifugo Token
Go 后端为用户签发 Centrifugo Connection JWT。

流程：
React
  |
  | GET /api/im/realtime/token
  v
Go
  |
  | 验证业务 JWT
  | 读取 userID
  | 生成 Centrifugo JWT
  v
React
  |
  | 使用 JWT 连接 Centrifugo
  v
WebSocket Connected


Go 代码示意：
type RealtimeTokenService struct {
    secret []byte
}

type RealtimeClaims struct {
    UserID string `json:"sub"`
    Exp    int64  `json:"exp"`
}

func (s *RealtimeTokenService) Generate(
    userID string,
) (string, error) {

    now := time.Now()

    claims := jwt.MapClaims{
        "sub": userID,
        "exp": now.Add(30 * time.Minute).Unix(),
    }

    token := jwt.NewWithClaims(
        jwt.SigningMethodHS256,
        claims,
    )

    return token.SignedString(s.secret)
}


这是 JWT 的基本示意，生产环境还应配置 Centrifugo 对应的签名密钥、算法和连接 Token 认证参数。

另外，不能把 Centrifugo API Key 或 JWT 签名密钥下发给 React。
3.4 React 连接代码
安装：
npm install centrifuge


连接：
import { Centrifuge } from "centrifuge";

export function createRealtimeClient(token: string) {

    const centrifuge = new Centrifuge(
        "wss://im.example.com/connection/websocket",
        {
            token,
        }
    );

    centrifuge.on("connected", (ctx) => {
        console.log("连接成功", ctx);
    });

    centrifuge.on("disconnected", (ctx) => {
        console.log("连接断开", ctx);
    });

    centrifuge.connect();

    return centrifuge;
}


订阅会话：
const sub = centrifuge.newSubscription(
    `conversation:${conversationId}`
);

sub.on("publication", (ctx) => {

    const event = ctx.data;

    if (event.eventType === "message.created") {
        console.log("收到新消息", event.data);
    }

});

sub.subscribe();


注意：这里只演示实时接收。实际项目中，用户进入会话前，必须先通过 Go API 获取授权会话信息，不能因为前端知道 conversationId 就允许订阅。
3.5 消息推送事件格式
建议定义统一事件结构：
type IMEvent struct {
    EventID        string          `json:"eventId"`
    EventType      string          `json:"eventType"`
    EventVersion   int             `json:"eventVersion"`

    ConversationID string          `json:"conversationId"`

    ActorUserID    string          `json:"actorUserId"`

    MessageID      string          `json:"messageId,omitempty"`

    OccurredAt     int64           `json:"occurredAt"`

    Data            json.RawMessage `json:"data"`
}


消息创建事件：
{
  "eventId": "EVT10001",
  "eventType": "message.created",
  "eventVersion": 1,
  "conversationId": "C10001",
  "actorUserId": "U10001",
  "messageId": "MSG10001",
  "occurredAt": 1793370000000,
  "data": {
    "messageId": "MSG10001",
    "messageSeq": 100,
    "messageType": 1,
    "content": {
      "text": "你好"
    }
  }
}


其他事件类型：
message.created
message.recalled
message.edited
message.read
message.delivered
conversation.created
conversation.updated
conversation.member_joined
conversation.member_left
user.online
user.offline
call.invited
call.accepted
call.rejected
call.ended


建议统一使用 eventType + eventVersion，方便未来增加事件字段而不影响旧版本 React 客户端。

四、消息发送：必须实现可靠投递
这是 IM 系统最关键的地方。
4.1 为什么需要 Outbox？
假设 Go 业务执行：
1. INSERT INTO im_message
2. 调用 Centrifugo 发布消息


如果：
MySQL 写入成功
Centrifugo 发布失败


数据库有消息，但是用户没有收到实时推送。

反过来，如果先推送，再写数据库：
Centrifugo 推送成功
MySQL 写入失败


用户收到了实际上没有持久化的消息。

因此需要 Outbox 模式。
4.2 消息事务
BEGIN TRANSACTION
    |
    +-- 1. 校验会话成员
    |
    +-- 2. 检查 client_message_id
    |
    +-- 3. 分配 message_seq
    |
    +-- 4. INSERT im_message
    |
    +-- 5. UPDATE im_conversation
    |
    +-- 6. UPDATE 成员未读数
    |
    +-- 7. INSERT im_message_outbox
    |
COMMIT
    |
    v
Outbox Worker
    |
    v
Kafka
    |
    v
Realtime Consumer
    |
    v
Centrifugo Publish


4.3 消息序号如何生成？
不建议使用：
SELECT MAX(message_seq) + 1
FROM im_message
WHERE conversation_id = ?;


并发情况下会发生序号冲突。

建议创建会话序号表：
CREATE TABLE im_conversation_sequence (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    conversation_id VARCHAR(64) NOT NULL,

    current_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,

    PRIMARY KEY (id),

    UNIQUE KEY uk_conversation (
        conversation_id
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


在事务中：
UPDATE im_conversation_sequence
SET current_seq = LAST_INSERT_ID(current_seq + 1)
WHERE conversation_id = ?;


然后：
SELECT LAST_INSERT_ID();


取得当前事务连接上的新序号。

注意：必须在同一个数据库连接、同一个事务中执行，不能将两个 SQL 分配给不同连接。

这样可以保证同一会话内消息序号严格递增。

代价是同一个会话的序号行会形成串行化热点。因此，普通群聊适用；超大群需要进一步设计消息分区、批量序号分配和写入队列，不能直接假设单行锁能无限扩展。
4.4 Go 消息服务接口
建议：
type SendMessageCommand struct {
    ConversationID string
    SenderID        string
    ClientMessageID string
    MessageType     int
    Content         json.RawMessage
    ReplyMessageID  string
}

type SendMessageResult struct {
    MessageID      string
    ConversationID string
    MessageSeq     uint64
    CreatedAt      time.Time
}

type MessageService interface {
    Send(
        ctx context.Context,
        command SendMessageCommand,
    ) (*SendMessageResult, error)

    Recall(
        ctx context.Context,
        messageID string,
        userID string,
    ) error

    GetHistory(
        ctx context.Context,
        conversationID string,
        beforeSeq uint64,
        limit int,
    ) ([]Message, error)
}


接口设计中，client_message_id 必须由 React 每次发送时生成，并且在重试时保持不变。

同一个用户重复提交相同 client_message_id，Go 必须返回第一次创建的消息，而不是创建第二条消息。

五、SeaweedFS 文件存储设计
5.1 为什么使用 SeaweedFS？
SeaweedFS 负责存储 IM 中的二进制文件：
- 图片：JPEG、PNG、WebP。
- 视频：MP4、MOV。
- 语音：AAC、MP3、Opus。
- 普通文件：PDF、DOCX、ZIP 等。
推荐通过 SeaweedFS 的 S3 兼容接口访问文件，而不是让业务模块直接操作 Volume Server。
                  React
                    |
                    | 1. 请求上传凭证
                    v
                Go Media API
                    |
                    | 2. 生成上传授权
                    v
                SeaweedFS S3
                    ^
                    |
                    | 3. 上传文件
                    |
                  React

                上传完成
                    |
                    v
                Go Media API
                    |
                    +---- 文件元数据写入 MySQL
                    |
                    +---- 校验文件状态
                    |
                    v
                返回 attachmentId
                    |
                    v
             SendMessage API
                    |
                    v
                im_message


SeaweedFS 提供 Filer、Volume 和 S3 等访问方式，普通业务客户端优先使用 Filer 或 S3 接口。

GitHub

5.2 文件上传采用预签名 URL
不推荐：
React -> Go -> SeaweedFS -> Go -> React


因为大文件会占用 Go API 的网络连接和内存资源。

推荐：
React -> Go：申请上传凭证
React -> SeaweedFS：直接上传
React -> Go：确认上传完成


Go API：
POST /api/im/media/upload/init
POST /api/im/media/upload/complete
GET  /api/im/media/{attachmentId}
DELETE /api/im/media/{attachmentId}


上传初始化请求：
{
  "fileName": "photo.jpg",
  "mimeType": "image/jpeg",
  "fileSize": 1024000,
  "fileHash": "sha256..."
}


返回：
{
  "attachmentId": "ATT10001",
  "uploadUrl": "https://storage.example.com/...",
  "method": "PUT",
  "expiresIn": 600
}


这里的 URL 是服务端生成的预签名地址，不能让前端自行拼接存储路径。
5.3 Go 文件存储接口抽象
建议不要在业务代码中直接调用 SeaweedFS SDK 或 S3 SDK。

定义统一接口：
type ObjectStorage interface {

    CreateUploadURL(
        ctx context.Context,
        objectKey string,
        contentType string,
        expires time.Duration,
    ) (string, error)

    CreateDownloadURL(
        ctx context.Context,
        objectKey string,
        expires time.Duration,
    ) (string, error)

    Delete(
        ctx context.Context,
        objectKey string,
    ) error

    Head(
        ctx context.Context,
        objectKey string,
    ) (*ObjectInfo, error)
}


目录结构：
internal/media/
├── service/
│   ├── upload.go
│   ├── download.go
│   └── attachment.go
│
├── storage/
│   ├── storage.go
│   ├── seaweedfs_s3.go
│   └── mock.go
│
├── repository/
│   └── attachment_repository.go
│
└── model/
    └── attachment.go


这样未来如果更换为 MinIO、AWS S3 或其他对象存储，不需要修改消息业务逻辑。
5.4 上传状态机
文件上传不要只有成功和失败两种状态。
INIT
  |
  v
UPLOADING
  |
  v
UPLOADED
  |
  v
VERIFYING
  |
  +------> FAILED
  |
  v
READY
  |
  v
ATTACHED
  |
  v
DELETED


说明：
状态	含义
INIT	已创建上传任务
UPLOADING	上传中
UPLOADED	客户端声明上传完成
VERIFYING	服务端验证文件
READY	文件可正常使用
ATTACHED	已关联消息
FAILED	验证或上传失败
DELETED	已删除

重要安全措施：
1. 验证文件真实大小，不相信客户端传入的 fileSize。
2. 验证 MIME 和文件实际格式。
3. 图片需要防止伪造扩展名。
4. 视频、图片可以通过异步 Worker 生成缩略图。
5. 未关联消息的文件设置过期清理策略。
6. 下载采用鉴权接口或短时效签名 URL。
7. 不允许用户通过猜测 objectKey 访问其他用户的私有文件。

六、LiveKit 音视频系统设计
6.1 LiveKit 的职责
LiveKit 不负责聊天消息的存储。

它只负责实时音视频媒体传输。
                  Go Backend
                      |
             创建呼叫业务记录
                      |
             创建 LiveKit Room
                      |
              生成 Access Token
                      |
             +--------+--------+
             |                 |
             v                 v
          React A           React B
             |                 |
             +--------+--------+
                      |
                      v
                  LiveKit
                      |
               WebRTC 音视频


LiveKit 支持 Go 服务端 SDK，可以创建 Access Token，并通过 RoomService 管理房间和参与者。

GitHub+1

6.2 呼叫业务流程
以用户 A 呼叫用户 B 为例：
用户 A                 Go                 用户 B
  |                     |                    |
  | POST /calls         |                    |
  |-------------------->|                    |
  |                     |                    |
  |                     | 创建 call_session  |
  |                     |                    |
  |                     | 生成 LiveKit Room  |
  |                     |                    |
  |                     | Centrifugo Publish |
  |                     |------------------->|
  |                     |                    |
  |                     |              收到来电
  |                     |                    |
  |                     |<-------------------|
  |                     | POST /accept       |
  |                     |                    |
  |<--------------------|                    |
  | 返回 LiveKit Token  |                    |
  |                     |                    |
  +---------------------+--------------------+
                        |
                        v
                     LiveKit
                        |
                   WebRTC连接


6.3 呼叫接口设计
POST /api/im/calls
    创建呼叫

POST /api/im/calls/{callId}/accept
    接听呼叫

POST /api/im/calls/{callId}/reject
    拒绝呼叫

POST /api/im/calls/{callId}/cancel
    主叫取消

POST /api/im/calls/{callId}/leave
    退出房间

POST /api/im/calls/{callId}/end
    结束呼叫

GET /api/im/calls/{callId}
    查询呼叫状态

POST /api/im/calls/{callId}/token
    获取 LiveKit Token


6.4 呼叫状态机
             CREATED
                |
                v
             RINGING
                |
       +--------+---------+
       |        |         |
       v        v         v
   ACCEPTED  REJECTED  CANCELLED
       |
       v
    CONNECTING
       |
       v
     ACTIVE
       |
       v
      ENDED


实际实现还需要处理：
- 呼叫超时。
- 对方离线。
- 用户忙线。
- 多设备同时接听。
- 主叫取消与接听同时发生。
- 网络中断。
- LiveKit 房间异常结束。
例如用户 B 同时使用手机和 Web 登录，只允许其中一个设备成功接听，需要在 MySQL 事务中通过条件更新控制状态迁移，不能单纯依赖前端按钮禁用。
6.5 LiveKit Go Token 示例
安装：
go get github.com/livekit/server-sdk-go/v2


Go：
package livekit

import (
    "time"

    "github.com/livekit/protocol/auth"
)

type TokenService struct {
    APIKey    string
    APISecret string
}

func (s *TokenService) GenerateJoinToken(
    roomName string,
    userID string,
) (string, error) {

    token := auth.NewAccessToken(
        s.APIKey,
        s.APISecret,
    )

    grant := &auth.VideoGrant{
        RoomJoin: true,
        Room:     roomName,
    }

    token.SetVideoGrant(grant).
        SetIdentity(userID).
        SetValidFor(10 * time.Minute)

    return token.ToJWT()
}


注意：
- API Key 和 API Secret 只放在 Go 服务端。
- 每次用户加入房间之前，Go 都要验证呼叫参与者身份。
- Token 必须绑定指定 Room 和 Identity。
- 不允许普通用户自行指定其他人的 Identity。
- 房间生命周期由 Go 管理，不能只依赖前端关闭页面。
6.6 React LiveKit
安装：
npm install livekit-client @livekit/components-react @livekit/components-styles


React：
import {
  LiveKitRoom,
  VideoConference,
} from "@livekit/components-react";

import "@livekit/components-styles";

interface Props {
  serverUrl: string;
  token: string;
}

export function CallRoom({
  serverUrl,
  token,
}: Props) {

  return (
    <LiveKitRoom
      serverUrl={serverUrl}
      token={token}
      connect={true}
      audio={true}
      video={true}
    >
      <VideoConference />
    </LiveKitRoom>
  );
}


其中 token 必须由 Go 接口签发。

七、Go API 接口完整规划
建议统一使用 /api/im 前缀。
7.1 会话接口
Method	URL	说明
POST	/conversations/single	创建或获取单聊
POST	/conversations/group	创建群聊
GET	/conversations	获取会话列表
GET	/conversations/{id}	会话详情
PATCH	/conversations/{id}	修改会话信息
DELETE	/conversations/{id}	删除或退出会话
POST	/conversations/{id}/members	添加成员
DELETE	/conversations/{id}/members/{userId}	移除成员
GET	/conversations/{id}/members	成员列表

7.2 消息接口
Method	URL	说明
POST	/messages	发送消息
GET	/messages/{id}	消息详情
GET	/conversations/{id}/messages	历史消息
POST	/messages/{id}/recall	撤回消息
PATCH	/messages/{id}	编辑消息
POST	/messages/{id}/read	已读回执
POST	/messages/{id}/reaction	添加表情
DELETE	/messages/{id}/reaction	删除表情

历史消息采用游标分页：
GET /api/im/conversations/C10001/messages?beforeSeq=200&limit=50


响应：
{
  "items": [
    {
      "messageId": "MSG10001",
      "messageSeq": 199,
      "senderId": "U10001",
      "messageType": 1,
      "content": {
        "text": "你好"
      }
    }
  ],
  "nextCursor": "199",
  "hasMore": true
}


不推荐大规模消息列表采用传统 OFFSET 分页。
7.3 实时接口
Method	URL	说明
GET	/realtime/token	Centrifugo Token
GET	/sync	登录后的数据同步
GET	/presence/{userId}	用户在线状态
POST	/heartbeat	业务心跳（按需）

7.4 音视频接口
使用前面定义的 /calls 接口。

另外还需要：
POST /api/im/webhooks/livekit


用于接收 LiveKit 服务端事件，例如：
room_started
room_finished
participant_joined
participant_left
track_published


Webhook 必须验证 LiveKit 签名，不能信任未经验证的 HTTP 请求。

八、React 前端架构
建议不要把所有聊天功能堆积在一个 Chat.tsx 文件中。
src/
├── modules/
│   └── im/
│       ├── api/
│       │   ├── conversation.ts
│       │   ├── message.ts
│       │   ├── media.ts
│       │   └── call.ts
│       │
│       ├── components/
│       │   ├── ConversationList/
│       │   ├── MessageList/
│       │   ├── MessageItem/
│       │   ├── MessageEditor/
│       │   ├── ImageMessage/
│       │   ├── VideoMessage/
│       │   ├── VoiceMessage/
│       │   └── FileMessage/
│       │
│       ├── hooks/
│       │   ├── useIMClient.ts
│       │   ├── useConversation.ts
│       │   ├── useMessages.ts
│       │   ├── useRealtime.ts
│       │   └── useCall.ts
│       │
│       ├── realtime/
│       │   ├── CentrifugoClient.ts
│       │   ├── EventDispatcher.ts
│       │   └── ReconnectManager.ts
│       │
│       ├── stores/
│       │   ├── conversationStore.ts
│       │   ├── messageStore.ts
│       │   └── callStore.ts
│       │
│       ├── call/
│       │   ├── CallProvider.tsx
│       │   ├── IncomingCall.tsx
│       │   ├── OutgoingCall.tsx
│       │   └── CallRoom.tsx
│       │
│       └── pages/
│           ├── IMHomePage.tsx
│           ├── ChatPage.tsx
│           └── CallPage.tsx


8.1 前端消息状态
建议每条消息维护客户端状态：
type MessageStatus =
  | "sending"
  | "sent"
  | "delivered"
  | "read"
  | "failed";


例如：
点击发送
   |
   v
sending
   |
   | Go 返回持久化成功
   v
sent
   |
   | 接收端确认收到
   v
delivered
   |
   | 接收端已读
   v
read


注意这几个状态含义不同：
- sent：服务端已经持久化。
- delivered：接收端设备已经确认收到。
- read：接收端已经阅读。
如果不要求逐设备送达确认，可以简化为服务端已持久化、已读两种状态。
8.2 React 多端消息去重
假设：
React A 发送 MSG100
Go 保存 MSG100
Centrifugo 推送 MSG100
React A 自己也收到 MSG100


React A 可能先收到 HTTP 响应，又收到 Centrifugo 推送。

因此前端需要通过 messageId 去重：
function mergeMessage(
  messages: Message[],
  incoming: Message
): Message[] {

  const index = messages.findIndex(
    item =>
      item.messageId === incoming.messageId ||
      item.clientMessageId === incoming.clientMessageId
  );

  if (index >= 0) {
    const next = [...messages];
    next[index] = {
      ...next[index],
      ...incoming,
    };
    return next;
  }

  return [...messages, incoming];
}


实际生产中还需要按 messageSeq 排序，并处理旧消息补拉与实时消息同时到达的情况。

九、离线消息与多端同步
这是对标成熟 IM 产品时必须重点实现的能力。
9.1 用户上线流程
用户登录
   |
   v
Go 验证 JWT
   |
   v
React 获取 Centrifugo Token
   |
   v
连接 Centrifugo
   |
   v
订阅个人频道 + 会话频道
   |
   v
GET /api/im/sync
   |
   +---- 获取会话列表
   |
   +---- 获取未读数
   |
   +---- 获取消息同步游标
   |
   v
同步离线数据
   |
   v
正常接收实时消息


建议先建立实时订阅，再执行同步接口，避免同步期间产生的消息漏收。
9.2 消息同步游标
可以为每个用户、每个会话维护：
conversation_id
user_id
last_read_seq
last_sync_seq
unread_count


这里需要区分：
- last_read_seq：用户已经阅读到的位置。
- last_sync_seq：客户端已经成功同步到的位置。
二者不能混为一谈。

例如：
服务器消息：

1  你好
2  在吗
3  图片
4  文件
5  好的

用户上次同步到：2
用户上次阅读到：1


用户重新上线：
同步 3、4、5

last_sync_seq = 5
last_read_seq = 1
unread_count = 4


消息已同步不等于消息已读。
9.3 Centrifugo Recovery 与数据库补偿
建议采用两层机制：
第一层：Centrifugo Recovery
    |
    +---- 短时间断线
    +---- 自动恢复遗漏的实时事件

第二层：Go + MySQL Sync
    |
    +---- 长时间离线
    +---- History 过期
    +---- Recovery 失败
    +---- 客户端游标丢失


Centrifugo 的 Recovery 可以通过 epoch、offset 判断是否能够恢复；如果无法恢复，前端必须重新调用 Go 同步接口。

Centrifugo+1


十、Redis 与 Kafka 的职责边界
你现有 Go 工程已经有 Kafka、Redis、MySQL 的基础设施，建议沿用现有组件，不需要为 IM 再引入一套重复基础设施。
组件	IM 中的用途	是否权威
MySQL	消息、会话、成员、附件元数据	权威
Redis	在线状态、临时缓存、限流	非权威
Kafka	消息事件异步分发	非权威
Centrifugo	在线消息推送及短期恢复	非权威
SeaweedFS	文件内容	文件存储权威
LiveKit	音视频实时媒体	媒体服务权威

10.1 Redis Key 规范
im:online:{userId}
im:device:{userId}:{deviceId}

im:conversation:{conversationId}

im:unread:{userId}:{conversationId}

im:typing:{conversationId}:{userId}

im:call:{callId}

im:rate:send:{userId}


其中：
- 在线状态设置 TTL。
- 输入状态设置短 TTL。
- 未读数可作为缓存，但 MySQL 或可靠的业务事件必须能重建。
- 不使用 Redis List 代替正式消息历史。
10.2 Kafka Topic 规范
im_message_events_v1
im_conversation_events_v1
im_read_events_v1
im_call_events_v1
im_media_events_v1


Kafka 分区建议：
partitionKey = conversation_id


这样同一个会话的消息事件尽量进入同一个分区，便于保持分区内消费顺序。

不过，Kafka 的顺序保证只存在于单个分区，不能把不同会话之间的消息顺序当作全局顺序。
10.3 可靠消费
沿用你已有的 Inbox 去重思路：
Kafka Event
    |
    v
Inbox Insert
    |
    +---- 已消费 -> 跳过
    |
    v
执行业务
    |
    v
Commit


建议事件消费也采用：
- event_id 唯一约束。
- Kafka Topic、Partition、Offset 记录。
- 失败重试。
- 死信队列。
- 消费延迟指标。
- Outbox 积压监控。
这里要特别强调：Kafka 至少一次投递与业务幂等结合，才能实现可靠的最终效果。不能简单宣称 Kafka 可以保证端到端 exactly-once。

十一、权限与安全设计
建议复用你现有 Go 工程中的 JWT + RBAC 权限体系。

但 IM 需要额外增加资源级权限判断。

例如：
权限	说明
im:conversation:create	创建会话
im:conversation:view	查看会话
im:message:send	发送消息
im:message:recall	撤回消息
im:message:read	读取消息
im:group:create	创建群组
im:group:manage	管理群组
im:media:upload	上传文件
im:call:create	发起呼叫
im:call:join	加入音视频房间

这里还要区分两个概念：

RBAC 解决用户具备什么功能权限，资源授权解决用户能否访问某个具体会话。

例如：
用户 A
  |
  +-- 有 im:message:send
  |
  +-- 是 C10001 的成员
  |       |
  |       +-- 可以发送消息
  |
  +-- 不是 C10002 的成员
          |
          +-- 禁止发送消息


每个消息、附件、会话和音视频房间都必须执行资源级授权。

其他安全措施：
1. HTTP API 使用 JWT 鉴权。
2. WebSocket 使用独立的 Centrifugo Token。
3. LiveKit Token 由 Go 签发。
4. SeaweedFS 不暴露管理端口。
5. 上传下载进行用户权限校验。
6. 消息发送实施用户、IP、会话三级限流。
7. Webhook 验证签名及防重放。
8. 消息内容进行 XSS 防护。
9. 敏感信息按业务要求加密。
10. 记录消息撤回、群成员变更、管理员操作审计日志。

十二、部署架构
建议使用 Docker Compose 作为开发环境，生产环境逐步转向 Kubernetes 或其他容器编排方案。
12.1 开发环境
Docker Compose
|
+-- mysql:8.0
|
+-- redis
|
+-- kafka
|
+-- centrifugo
|
+-- livekit
|
+-- seaweedfs
|    |
|    +-- master
|    +-- volume
|    +-- filer
|    +-- s3
|
+-- go-api
|
+-- go-im-worker
|
+-- react-web


12.2 生产环境
                    Internet
                       |
                 Load Balancer
                       |
              +--------+--------+
              |                 |
              v                 v
           Go API           Centrifugo
          多实例部署          多实例部署
              |                 |
              |                 |
        +-----+------+          |
        |            |          |
        v            v          v
      MySQL         Redis      Redis
     主从集群       集群       Broker
        |
        v
      Kafka
     多 Broker
        |
        v
   Go IM Workers
        |
        +----------------------+
        |                      |
        v                      v
    Centrifugo              SeaweedFS
                               |
                         Master/Filer
                               |
                          Volume 集群

                   LiveKit 集群
                       |
                   WebRTC 媒体
                       |
                   TURN 服务


注意 LiveKit 的 UDP 媒体端口、TURN/STUN 配置与普通 HTTP/WebSocket 负载均衡不同，需要单独规划公网网络和防火墙。

另外：
- Centrifugo 多实例需要配置合适的 Broker 和 Presence Manager。
- SeaweedFS 需要规划副本策略、磁盘容量、故障恢复和备份。
- MySQL 需要规划主从复制、备份及消息数据归档。
- Kafka 需要监控消费积压。
- LiveKit 需要根据并发房间数、参与者数和带宽规划节点。
不要把所有 WebSocket 长连接都交给 Go API 自己维护，Centrifugo 就是专门负责这一层的。

十三、千万级、亿级数据的扩展设计
这里需要区分三个指标：
- 用户总量：注册用户数。
- 在线并发：同时连接的用户数。
- 消息吞吐：每秒产生和分发的消息数量。
三者不是同一个概念。

建议提前设计以下扩展点：
模块	扩展方案
消息表	按时间分表或按会话 ID 分片
会话表	按用户访问模式设计索引
消息序号	会话级序号，热点会话独立优化
消息推送	Centrifugo 水平扩容
事件消费	Kafka 分区 + Consumer Group
在线状态	Redis 集群 + TTL
图片视频	SeaweedFS 多 Volume
音视频	LiveKit 多节点部署
历史消息	游标分页 + 冷热数据分层

特别注意：群聊扇出问题
假设：
一个群有 10 万人。

群内用户 A 发送一条消息。


如果直接执行：
INSERT 10 万条用户消息记录
UPDATE 10 万个用户未读数
PUBLISH 10 万次


很容易造成写放大。

建议：
- 消息正文只存一份。
- 通过 conversation_id + message_seq 共享消息流。
- 用户未读数采用异步聚合或游标计算。
- 大群采用分批扇出。
- 对在线成员推送实时事件。
- 离线成员通过同步接口获取消息。
- 超大群单独设计推送分片与限流策略。
千万用户不代表每条消息都要向千万用户推送。 必须依据实际会话成员关系进行消息分发。

十四、建议的开发实施顺序
不建议一开始就把所有模块同时开发。按照以下阶段推进，能够形成真正可运行的产品。
第一阶段：IM 基础能力
优先实施
- [ ] Go 用户鉴权
- [ ] 单聊、群聊
- [ ] 会话成员管理
- [ ] 文本消息发送
- [ ] MySQL 消息持久化
- [ ] 消息幂等
- [ ] Centrifugo 实时推送
- [ ] 消息历史游标分页
- [ ] 离线消息同步
  
  
  第二阶段：完整消息中心
- [ ] SeaweedFS 图片上传
- [ ] 视频、语音、文件消息
- [ ] 消息已读回执
- [ ] 未读数
- [ ] 多端同步
- [ ] 消息撤回
- [ ] 群管理
- [ ] 消息回复、表情
  
  
  第三阶段：音视频
- [ ] LiveKit 房间管理
- [ ] 一对一语音通话
- [ ] 一对一视频通话
- [ ] 多人语音会议
- [ ] 多人视频会议
- [ ] 屏幕共享
- [ ] 通话记录
- [ ] 异常断线恢复
  
  
  第四阶段：生产级优化
- [ ] Kafka Outbox / Inbox
- [ ] 大群消息扇出
- [ ] 消息分库分表
- [ ] Redis 集群
- [ ] Centrifugo 集群
- [ ] SeaweedFS 容灾
- [ ] LiveKit 集群
- [ ] OpenTelemetry
- [ ] Prometheus + Grafana
- [ ] 压力测试及故障演练

十五、关键技术组件版本与官方资料
以下是建议采用的组件及官方资料，具体版本应在项目初始化时锁定并进行兼容性测试。
技术	官方资料
Go	Go Documentation
Centrifugo	Centrifugo Documentation
Centrifugo React SDK	Client SDK
LiveKit Go SDK	LiveKit Go SDK
LiveKit React	LiveKit Documentation
SeaweedFS	SeaweedFS REST API


十六、最终技术选型建议
我建议你最终采用下面这套组合：
层次	选型	设计原则
前端	React + TypeScript	组件化、状态管理
实时通信	Centrifugo	WebSocket、Pub/Sub
音视频	LiveKit	WebRTC
文件存储	SeaweedFS	S3 兼容
后端	Go	模块化单体
数据库	MySQL 8	消息权威数据
缓存	Redis	在线状态、缓存
消息队列	Kafka	异步事件
可靠性	Outbox + Inbox	幂等、重试
监控	OpenTelemetry + Prometheus	全链路观测

有一个架构决策尤其重要：不要把 IM 消息、系统通知、业务通知混成一张消息表。

建议划分为三个领域：
Communication（通信中心）
    |
    +-- IM
    |    +-- 单聊
    |    +-- 群聊
    |    +-- 图片、视频、文件
    |    +-- 音视频通话
    |
    +-- Notification（通知中心）
    |    +-- 系统通知
    |    +-- 业务通知
    |    +-- 消息模板
    |    +-- 站内信
    |    +-- 短信、邮件、Push
    |
    +-- Realtime（实时基础设施）
         +-- Centrifugo
         +-- Presence
         +-- Event Dispatch


IM 是用户之间的通信，Notification 是平台向用户发送的通知。两者可以共用 Centrifugo，但业务数据模型、发送规则、生命周期应该独立。

这也方便后续与你正在设计的通知模板、通知渠道、消息中心整合。
十七、下一步建议：先确定工程落地边界
你提供的 ChatGPT 分享链接，我目前只能读取到登录页面，无法读取其中的实际对话内容，因此还不能保证与链接中的具体需求逐项兼容。上面的方案是按照 Go + Centrifugo + LiveKit + SeaweedFS 独立 IM 系统设计的。

建议下一步直接形成一套可交给 Codex/OpenCode 执行的工程代码规范：
IM/
├── backend/
│   ├── internal/
│   ├── migrations/
│   ├── configs/
│   ├── deployments/
│   └── tests/
│
├── frontend/
│   └── src/modules/im/
│
├── infrastructure/
│   ├── centrifugo/
│   ├── livekit/
│   ├── seaweedfs/
│   ├── kafka/
│   └── redis/
│
├── docs/
│   ├── architecture.md
│   ├── database.md
│   ├── api.md
│   ├── events.md
│   └── deployment.md
│
└── docker-compose.yml


我建议第一步先完成数据库 DDL、Go 接口与 Service/Repository、Outbox 消息可靠投递、Centrifugo 实时推送，以及 React 聊天页面，跑通完整的单聊闭环，再扩展群聊和音视频。

如果把分享链接中的内容粘贴过来，我可以进一步将本方案与原需求合并，形成一份没有模块冲突、可以直接按目录逐个编码实现的最终工程设计。
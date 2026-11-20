
- [kafka中Topic、Partition、Offset解读](#kafka中Topic、Partition、Offset解读)
- [现有工程实战：从启动、接口入队，到消费完成](#四现有工程实战从启动接口入队到消费完成)

***
<br/><br/><br/>
># <h2 id="kafka中Topic、Partition、Offset解读">kafka中Topic、Partition、Offset解读</h2>


## Outbox 事件是什么？

这里使用的是 Transactional Outbox（事务发件箱）模式。

它解决的问题是：数据库写入成功，但 Kafka 消息发送失败怎么办？

```go
hg_interaction_consumer.go 中有

KafkaTopic     string // Kafka Topic 名称，表示消息来自哪个主题
KafkaPartition int32  // Kafka 分区编号，表示消息位于哪个分区
KafkaOffset    int64  // Kafka 消息偏移量，表示消息在所属分区中的位置
```

有用，而且这三个字段是 Kafka 定位消息、管理消费进度、实现消息去重的重要信息。

## 一、三个字段分别是什么？

假设你的 Go 服务使用 Kafka 接收用户点赞事件。

```
Kafka 集群
    |
    +-- Topic: video-interaction
          |
          +-- Partition 0
          |     +-- Offset 0 → 用户A点赞
          |     +-- Offset 1 → 用户B点赞
          |     +-- Offset 2 → 用户C点赞
          |
          +-- Partition 1
          |     +-- Offset 0 → 用户D点赞
          |     +-- Offset 1 → 用户E点赞
          |
          +-- Partition 2
                +-- Offset 0 → 用户F点赞
                +-- Offset 1 → 用户G点赞
```

| 字段               | 含义             | 示例                  |
| ---------------- | -------------- | ------------------- |
| `KafkaTopic`     | 消息所属的主题，类似消息分类 | `video-interaction` |
| `KafkaPartition` | Topic 中的分区编号   | `1`                 |
| `KafkaOffset`    | 消息在该分区中的位置编号   | `0`                 |

三个字段组合起来，可以唯一定位 Kafka 日志中的一条消息。

例如：

```
KafkaTopic     = "video-interaction"
KafkaPartition = 1
KafkaOffset    = 0
```

代表 `video-interaction` 主题的第 1 号分区中，Offset 为 0 的消息。

注意：Offset 只在所属分区内有意义，不是整个 Topic 的全局编号。

## 二、这三个字段有什么实际用途？

### 1. 消息去重

Kafka 消费者可能重复收到同一条消息。

例如：

```
第一次消费：
Topic=video-interaction
Partition=1
Offset=100
        |
        v
数据库处理成功

消费者尚未提交消费进度就崩溃
        |
        v
重新启动
        |
        v
再次消费 Offset=100
```

这时可以通过数据库中的 Topic、Partition、Offset 判断这条消息是否已经处理过。

你的 `hgClaimInbox()` 就使用了这个机制。

### 2. 追踪消息和排查问题

假设用户点赞后，数据库没有更新。

日志记录：

```
Topic     = video-interaction
Partition = 2
Offset    = 1050
```

开发人员可以根据这些信息定位 Kafka 中的消息，检查其 Payload、消费日志和处理结果。

前提是消息仍在 Kafka 的保留期内。

### 3. 管理 Kafka 消费进度

Kafka 消费者需要知道每个分区已经消费到哪里。

例如：

```
Partition 0：已处理到 Offset 100
Partition 1：已处理到 Offset 200
Partition 2：已处理到 Offset 300
```

消费者提交 Offset 后，Kafka 消费者组就可以根据提交的位置恢复消费。

需要区分：

- Kafka 提交的 Offset：用于消费者组恢复消费，通常提交下一条待消费消息的位置。
- 数据库 Inbox 保存的 Offset：用于业务去重和追踪，并不等于已经向 Kafka 提交消费进度。

## 三、KafkaOffset 和 EventID 有什么区别？

这是你的代码中最值得理解的地方。

| 对比     | EventID     | KafkaOffset             |
| ------ | ----------- | ----------------------- |
| 谁生成    | 通常由业务系统生成   | Kafka 分区日志分配            |
| 作用     | 唯一标识业务事件    | 定位 Kafka 消息             |
| 是否全局唯一 | 取决于业务 ID 设计 | 不是，需要结合 Topic、Partition |
| 重复投递时  | 通常保持不变      | 可能发生变化                  |
| 主要用途   | 业务幂等        | 消息定位、消费进度追踪             |

例如同一个点赞事件：

```
EventID = event_1001

第一次投递：
Topic=video-interaction
Partition=0
Offset=100

第二次重新生产：
Topic=video-interaction
Partition=0
Offset=105
```

虽然 Offset 不同，但 EventID 相同，说明可能是同一个业务事件被重新生产到 Kafka。

因此，你的 Go 工程同时保存：

```
event.EventID
event.KafkaTopic
event.KafkaPartition
event.KafkaOffset
```

是合理的设计。

总结：

- `Topic`：消息在哪个主题？
- `Partition`：消息在哪个分区？
- `Offset`：消息在分区的哪个位置？
- `EventID`：这到底是哪一个业务事件？

对于你的 Kafka + Go + MySQL 项目，推荐使用 EventID 做业务幂等，Topic + Partition + Offset 做 Kafka 消息定位和投递去重，两者结合更可靠。

---

## 四、现有工程实战：从启动、接口入队，到消费完成

以下以**视频点赞接口**为主线，按当前代码讲解；路径均相对项目根目录。先记住一句话：

> **接口负责把命令交给 Kafka，消费者负责落库，数据库提交成功后才提交 Kafka 消费进度。**

### 1. 先看全流程

```text
启动 Go 主服务
  ├─ 初始化共享 Kafka Producer
  ├─ 启动已启用的 Consumer 消费循环
  └─ 启动 Outbox Dispatcher 后台投递循环

客户端 POST /api/v1/video_interactions/like
  → API Gateway 限流/并发保护 → API Guard/JWT 校验
  → Handler.Like → Service.SetVideoInteraction
  → KafkaEventBus.Publish → EventEnvelope → ProduceSync
  → Kafka 确认收到消息
      ├─ 请求侧：尝试更新 Redis 乐观状态 → 返回 HTTP 202
      └─ 消费侧：PollRecords → 解码 → interaction.HandleBatch
                 → MySQL 事务：Inbox 去重 + 互动状态 + 分片计数
                 → MySQL Commit 成功
                 → Kafka CommitRecords 提交 offset
                 → 这一消费组的本批消息处理完成

另有后台 Reprojector：MySQL 权威结果 → Redis 状态/计数校准
```

生产和消费在时间上可以交错：消费者可能在 HTTP 返回前就处理完，也可能稍后才完成。**`202` 表示已接受，不表示 MySQL 已写完。**

### 2. 如何启动

#### 2.1 本机 debug 启动顺序

先启动 Docker Desktop，并准备项目现有本地配置、Go 和带 MySQL 驱动的 `migrate` 工具。在项目根目录执行：

```bash
# 此脚本会实际启动依赖、执行 debug 数据库迁移、创建并检查 Topic。
./scripts/ensure_debug_deps.sh debug

# 上一步成功后，启动业务主服务，并要求 Kafka 必须可用。
SERVER_ENV=debug KAFKA_REQUIRED=true go run -tags production .
```

- 脚本依次检查/启动 MySQL、执行 debug 迁移、检查/启动 Redis 和 ClickHouse，再检查三节点 Kafka、执行 `make kafka-init`。
- `-tags production` 选择业务入口；`SERVER_ENV=debug` 选择本地配置，两者含义不同。
- `hg_kafka_application.go` 的 `hgInitKafkaIfConfigured()` 当前允许 debug 在 broker 初始化失败时降级；设置 `KAFKA_REQUIRED=true` 可避免“HTTP 启动了，但 Kafka 链路没启动”。pre/prod 会要求 Kafka 初始化成功。
- 主服务会同时启动配置启用的后台任务。上述依赖准备脚本仅用于对应本地环境；pre/prod 的 Topic 由发布系统/IaC 管理。

如果 MySQL、Redis、数据库结构及其他启用模块依赖已就绪，只补 Kafka 的命令是：

```bash
docker compose -f deployments/docker-compose.kafka.yml up -d kafka-1 kafka-2 kafka-3
make kafka-init
```

**一个需要补齐的实际细节：**当前 `scripts/kafka_init.sh` 会准备领域事件、弹幕、采集流水线和 IM Topic，但没有创建业务死信 Topic `hg.dlq.business`；Compose 又关闭了自动建 Topic。本地验证失败处理前，还需显式准备它，例如：

```bash
docker exec mlc-kafka-1 /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server kafka-1:9092,kafka-2:9092,kafka-3:9092 \
  --create --if-not-exists --topic hg.dlq.business \
  --partitions 12 --replication-factor 3
```

这里的 12 分区是本地创建示例；已有 Topic 不会被此命令自动调整。没有可用 DLQ 时，终止型错误也不能安全跳过。

#### 2.2 启动后究竟执行了哪些方法

| 文件 | 方法/配置 | 做什么 |
| --- | --- | --- |
| `main_production.go`、`main_mlc_project.go` | `main()` → `mlc_main()` → `buildMLCApplication()` | 装配 Redis、MySQL、Kafka、业务模块及 HTTP 服务 |
| `hg_kafka_application.go` | `initKafkaWithDependencies()` | 初始化 Producer、消费运行时及 Outbox Dispatcher |
| `internal/pkg/config/hg_kafka_config.go` | `GetKafkaConfig()` | 读取并校验合并后的 Kafka 配置 |
| `internal/pkg/kafka/hg_client.go` | `HGInitKafka()` | 创建全局 Producer Client，启动期用 3 秒上限 Ping broker |
| `internal/infrastructure/kafka/hg_runtime.go` | `NewRuntime()` → `HGRuntime.Start()` | 每个启用的消费组创建独立 Client，并启动长期消费 goroutine |
| `internal/outbox/hg_outbox_dispatcher.go` | `Dispatcher.Run()` | 后台把 MySQL Outbox 中待发送的事件交给 Kafka |

默认配置重点：

- `config/base/kafka.yaml`：业务发送 `acks: all`。
- `config/debug/kafka.yaml`：宿主机 broker 地址为 `localhost:19092/29092/39092`；领域事件 Topic 为 `mlc.domain.events`；Interaction 已启用，组名为 `mlc-go-debug-interaction`。
- `scripts/kafka_init.sh`：默认领域事件 Topic 为 **12 分区、3 副本**；弹幕专用 Topic 默认 96 分区。最终以实际集群拓扑为准。
- Go 运行在宿主机时用 `localhost` 暴露端口；同一 Compose 网络内的工具用 `kafka-1:9092` 等内部地址。

**不需要每次调用接口时启动消费者，也不需要每次请求创建 Kafka Client。**当前主进程已经同时承担生产和消费；全局 Producer 与各消费组 Client 是不同的长期对象。

### 3. 一次点赞请求，如何发消息进入队列

#### 3.1 调用哪个接口

使用现有 App/Web 的请求封装，携带合法 JWT、设备及签名等公共请求头，调用：

```http
POST /api/v1/video_interactions/like
Content-Type: application/json

{"submissionId":"实际投稿ID","active":true}
```

这是业务请求体示意，公共请求头省略；只有一个 Bearer Token 不一定能通过现有 API Guard。取消点赞时传 `active:false`。用户 ID 从认证上下文读取，不由请求体指定。

#### 3.2 按文件追踪发送链路

| 顺序 | 文件 | 方法与重点 |
| --- | --- | --- |
| ① 路由 | `internal/pkg/hg_router/hg_route_video_interaction.go`、`hg_route_groups.go` | 注册 `/like`，`NewVideoInteractionRouteGroup()` 接入公共校验和 JWT |
| ② 解析请求 | `internal/modules/video_interaction/handler/hg_video_interaction_handler.go` | `Like()` → `action()`：读取当前用户、解析 JSON，Handler 请求体上限 8 KiB |
| ③ 生成事件 | `internal/modules/video_interaction/service/hg_video_interaction_service.go` | `SetVideoInteraction()`：校验参数，构建 `VideoInteractionChangedEvent`，调用 `eventBus.Publish()` |
| ④ 包装协议 | `internal/infrastructure/eventbus/hg_event_bus.go` | `KafkaEventBus.Publish()` → `newKafkaEnvelope()` |
| ⑤ 生成信封 | `internal/events/hg_event.go` | `NewEnvelope()`：生成 UUID `eventId`，附上事件名、key、版本及业务 payload |
| ⑥ 真正发送 | `internal/pkg/kafka/hg_client.go` | `HGSendBusinessEvent()` → `HGBuildRecord()` → `client.ProduceSync(ctx, record).FirstErr()` |
| ⑦ 返回接受 | 上述 Service / Handler | 发送成功后尝试 `cache.ApplyOptimistic()`，返回 `202`，业务响应包含 `accepted:true` |

模块装配在 `internal/modules/video_interaction/module/hg_video_interaction_module.go` 的 `RegisterModules()`：它注入 `NewKafkaEventBus("mlc.domain.events")`。因此**当前点赞是直接发 Kafka，并不先写 Outbox**；改消费者订阅 Topic 时也要核对这个生产端常量。

#### 3.3 队列里实际放的是什么

放的是 JSON `EventEnvelope`，核心字段如下：

| 字段 | 本例含义 |
| --- | --- |
| `eventId` | 本次事件 UUID；重复投递同一事件时应保持不变 |
| `eventName` | `video.interaction.changed`，告诉消费者这是哪类事件 |
| `eventKey` | `submissionId:actorUserId:like`，同时作为 Kafka Record 的 key |
| `version` | 当前为 `1`，用于协议兼容检查 |
| `payload` | 业务 JSON 对象，包含操作者、投稿 ID、action、active 等 |

key 定义在 `internal/events/interaction/hg_interaction_event.go` 的 `VideoInteractionChangedEvent.EventKey()`。在分区数及路由策略稳定时，同一个“视频 + 用户 + 操作”进入同一分区；不同用户的点赞可以分散到不同分区。

**同步发送与异步业务处理并不矛盾：**`ProduceSync()` 只等 Kafka 的生产确认，不等消费者写 MySQL。发送错误会向上传回接口，当前通常返回 `503`；确认超时也可能是“Kafka 已写入，但响应没收到”，不能据此断言消息不存在。

### 4. 消费者如何拿消息、处理，并确认结束

#### 4.1 当前 Interaction 实际走批处理入口

```text
HGRuntime.Start()
  → RunDomainEventConsumerWithLagObserver()
  → 检测到 interaction.Consumer 实现了 consumer.BatchHandler
  → HGBaseConsumer.HGRunConsumeBatch()
  → consumeBatchLoop()
  → client.PollRecords(ctx, 500)              拉取最多 500 条记录
  → 按 topic/partition 分批调用处理器
  → hgDomainEventBatchHandler()              解码每条信封并保留来源坐标
  → interaction.Consumer.HandleBatch()
  → hgMapEvent()                            筛选互动/关注事件并解析 payload
  → Repository.ApplyEvents()
```

对应文件：

- 消费组启动：`internal/infrastructure/kafka/hg_runtime.go`。
- Kafka Record 到业务信封的适配：`internal/infrastructure/kafka/hg_domain_event_runner.go`。
- 拉取、失败处理与 offset 提交：`internal/pkg/kafka/hg_consumer.go`。
- `DecodeEnvelope()` / 来源坐标定义：`internal/consumer/hg_domain_event_consumer.go`。
- 互动事件解析：`internal/consumer/interaction/hg_interaction_consumer.go`。

`Consumer.Handle()` 也存在，但这个消费者在当前 Runtime 下优先走 `HandleBatch()`。不是每条消息都新建一个 goroutine；当前一个消费组在一个进程内的消费循环，会依次处理本轮拿到的各分区批次。

#### 4.2 最关键的事务：去重和业务写入必须一起提交

在 `internal/modules/video_interaction/repository/hg_video_interaction_repository.go`：

```text
ApplyEvents()：每批最多 100 个事件
  → hgApplyBatch()：开启 MySQL 事务
      → hgClaimInbox()：登记/检查 video_interaction_inbox
          ├─ 已处理的相同事件：跳过业务副作用
          ├─ 事件身份或内容冲突：返回错误
          └─ 新事件：继续
      → hgApplyVideoInteraction()
          → 锁定并更新 video_user_interactions 中的点赞状态
          → 状态确实变化时，更新 video_interaction_stat_shards
      → tx.Commit()
```

- `video_interaction_inbox` 有 `event_id` 和 `(kafka_topic,kafka_partition,kafka_offset)` 两组唯一约束，见 `migrations/000011_create_video_interactions.up.sql`。
- `hgClaimInbox()` 不只是按 offset 去重：它会按 EventID 查已存记录，并核对事件名、key 和 JSON 内容；同 ID 不同内容会报冲突。
- Inbox、互动状态、计数在**同一个事务**内：业务失败，Inbox 也回滚，下一次才能正常重试。
- 点赞计数按用户散列到 **64 个 MySQL 统计分片**，减少热门视频所有请求争抢同一计数行。这 64 个分片与 Kafka 的 12 个分区是两种不同设计。
- SQL 集中在 `internal/pkg/mysql/queries/hg_video_interaction_queries.go`；每 100 个事件是一笔有界事务，内部仍有逐事件 SQL，不等于一条 SQL 批量完成全部写入。

例如本轮拉取 300 条，第 1 个 100 条事务已提交，第 2 个事务失败：第 1 批不会回滚；重读到它们时由 Inbox 去重。

#### 4.3 最后才提交 Kafka offset

业务处理成功返回 `nil` 后，`consumeBatchLoop()` 调用 `hgCommitAndRestore()`，其中执行 `client.CommitRecords()`。消费者配置在 `internal/pkg/kafka/hg_config_builder.go` 的 `HGNewBusinessConsumerOpts()`，已经关闭自动提交。

例：某分区处理完 offset `100、101、102`，传入最后成功记录 `102`，Kafka 提交位置为 **103（下一条待读位置）**。

| 情况 | 当前处理方式 |
| --- | --- |
| MySQL 成功，offset 提交成功 | 该消费组从后续位置继续 |
| MySQL 成功，offset 提交失败或进程崩溃 | 可能重复消费，Inbox 防止重复业务写入；批量提交失败会记录指标 |
| MySQL 等普通错误 | 尝试写 DLQ 留档，并用 `SetOffsets()` 回到失败位置；不会因为写入 DLQ 就跳过 |
| 已标记的终止型错误，例如批处理解码失败 | 只有 `HGSendDLQ()` 成功，才允许越过该坏消息；后续记录再继续处理 |
| DLQ 本身失败 | 保留失败位置重试，不能直接确认成功 |

`HGSendDLQ()` 在 `internal/pkg/kafka/hg_dlq.go`，业务死信写入 `hg.dlq.business`。当前通用消费循环没有重试次数上限/退避调度，普通错误持续存在时会反复重试，也可能产生重复 DLQ 留档。

**消费完成不会删除 Kafka 消息**：提交的是这个消费组的进度，消息仍受 Topic 保留/清理策略管理；其他消费组有自己的进度。

#### 4.4 Redis 和页面看到的结果

接口发送成功后调用 `internal/modules/video_interaction/cache/hg_video_interaction_cache.go` 的 `ApplyOptimistic()`，优先改善即时显示；其错误在 Service 中被忽略，不会撤销已进入 Kafka 的消息。

最终校准由 `internal/modules/video_interaction/task/hg_reprojector.go` 的 `HGReprojector.Start()` / `RunOnce()` 完成：按游标读取 MySQL 权威状态，重新投影到 Redis。`config/base/app.yaml` 默认开启，周期 1 分钟、安全延迟 5 秒；实际可见延迟还取决于消费积压和投影耗时，不能承诺固定 5 秒完成。

因此，`GET /api/v1/video_interactions/state` 读到点赞成功只是 Redis 当前视图，不能单独证明这次事件已经落库。当前点赞响应也没有返回 EventID/任务查询地址。

### 5. 如果接口还要先改数据库：走 Outbox

本工程的**投币接口** `/api/v1/video_interactions/coin` 是现成例子：扣币必须立即在 MySQL 事务里校验余额和幂等，后续互动投影再走 Kafka。

```text
Handler.Coin → Service.SetVideoInteraction（coin 分支）
  → coin.HGService.Debit()
  → coin.HGRepository.Mutate() → hgMutateOnce() → hgMutateTx()
      [同一 MySQL 事务]
      请求幂等校验 + 扣币/账务流水 + outbox.Repository.SaveTx()
      → Commit
  → 返回 202（权威扣币已提交，事件已写 Outbox，Kafka 可能尚未收到）

后台 Dispatcher.Run()
  → hgDispatchAvailable() → Repository.Claim()
  → KafkaByteProducer.Send() → HGSendBusinessEventBytes() → ProduceSync()
  → 成功后 MarkPublished()
  → Kafka interaction 消费者按前面的流程更新互动状态/计数
```

投币账务入口在 `internal/modules/coin/service/hg_coin_service.go`、`internal/modules/coin/repository/hg_coin_repository.go`；消费端更新互动投影，不再扣一次钱包余额。

Outbox 的几个关键点：

1. **必须传同一个 `tx`。**`SaveTx()` 在 `internal/outbox/hg_outbox_repository.go`。仅在业务提交后调用 `OutboxEventBus.Publish()` 并不等价：后者调用 `Save()`，不能把此前的业务事务补成原子事务。
2. **先落稳定信封，再发送。**`outbox_event.payload` 保存完整 Envelope JSON，重发复用同一个 EventID 和字节；用 `HGSendBusinessEventBytes()` 避免再次 JSON 编码成字符串。
3. **认领短事务，网络发送在事务外。**`Claim()` 用 `FOR UPDATE SKIP LOCKED` 取一批记录、写入租约 token 和到期时间，再提交；发送结束携带 token 更新，防止旧 worker 确认新租约。SQL 在 `internal/pkg/mysql/queries/hg_sql_queries.go`。
4. **并发有上限。**Dispatcher 默认 64 个发送 worker、30 秒租约。启动调用虽然传 batchSize=100，实际会截到 64；有积压立即处理下一批，无任务时等待约 1 秒轮询。
5. **失败可追踪。**发送失败走 `MarkRetry()`，状态仍为 `pending`，按 1、2、4…秒退避，延迟封顶 256 秒；第 10 次发送失败走 `MarkDead()`。Outbox 的 `dead` 是数据库状态，与 Kafka DLQ Topic 不同。
6. **允许重复，不保证端到端恰好一次。**Kafka 收到了但 `MarkPublished()` 失败时，租约到期可能重发；消费者仍须幂等。`Claim` 或状态更新报错会使当前 `Run()` 返回，启动层记录 `Outbox dispatcher stopped`，这里没有自动重启循环。

Outbox 解决的是“业务写入与待发事件一起保存”；它仍消耗请求侧数据库能力，Kafka 不能替它消除扣币事务和 Outbox 写入压力。

### 6. 高并发时，重点控制什么

#### 6.1 当前工程已有的边界

| 层级 | 当前实现/默认值 | 意义 |
| --- | --- | --- |
| HTTP 入口 | `config/base/app.yaml`：互动模块按“模块 + 来源 IP”共享令牌桶，容量 120、每秒补充 30；单实例最多 6000 个在途请求 | 限流返回 `429`，在途满载返回 `503`；被拒绝请求尚未进入 Kafka |
| Producer | `hg_config_builder.go`：缓存最多 100000 条或 512 MiB，业务批次上限 8 MiB，配置 linger=100ms，Produce 请求超时 10 秒 | 多请求复用 Client 聚合发送；缓存满会形成背压，单次 Produce 超时不等于整个 HTTP 的超时预算 |
| Kafka | 领域事件默认 12 分区 | 同组实例通过分配分区分担消费 |
| Consumer | 单次 Poll 最多 500 条；Interaction 数据库事务最多 100 个事件 | 控制内存、事务长度和重平衡等待 |
| MySQL | Inbox 唯一键 + 用户/视频/操作关系唯一键 + 64 分片计数 | 支撑幂等，降低热点计数行竞争 |
| Outbox | 每实例最多 64 条并发发送 | 限制后台发送并发，多个实例通过租约协作 |

这些是**代码/配置上限，不是已验证的 QPS 或容量承诺**。实际吞吐取决于消息大小、分区分布、broker、数据库连接池和索引；批次大小也要与 broker/Topic 的消息大小限制匹配。请求侧应有明确的 context 超时预算。

#### 6.2 正确扩容：同一业务用同一 group_id

```text
mlc.domain.events：12 个分区
  └─ group_id = mlc-go-debug-interaction
       ├─ 实例 A：分担部分分区
       ├─ 实例 B：分担部分分区
       └─ 实例 C：分担部分分区

另一个 group_id = mlc-go-debug-feed
  └─ 独立读取同一 Topic，按事件类型选择处理或忽略
```

- 同组同一分区通常只分配给一个消费者；仅订阅这个 12 分区 Topic 时，超过 12 个组成员不会继续增加分区消费并行度。
- **为了扩容而给每个实例换 group_id，会变成各组各消费一遍。**不同业务用途才使用不同消费组。
- 当前一个进程内，每个组是一个消费循环，多个分区的业务处理并非自动多 goroutine 并行；扩实例前还需评估 MySQL 总连接数和热点锁。
- Kafka 保证的是分区日志顺序，不是用户点击时间顺序。并发 HTTP、多个生产者或 Outbox 64 路并发，都不能仅凭“相同 key”保证业务先后；扩分区也可能改变 key 映射。

#### 6.3 三个容易误解的点

1. **请求重试 ≠ 消息重投。**重新调用点赞 API 会再次执行 `NewEnvelope()`，产生新 EventID；Inbox 不能把两次 HTTP 请求直接合并。点赞用 `active` 绝对状态减少重复加计数；投币则另用用户 + `requestId` 做业务请求幂等。新增扣费、发券等接口应明确自己的请求幂等契约。
2. **`acks=all` ≠ 永远等三副本。**它等当前 ISR 确认，可写副本下限还受业务 Topic 的 `min.insync.replicas` 约束；本地 Compose 里的事务状态 Topic 最小 ISR 配置不能代替业务 Topic 配置。
3. **Kafka 能缓冲短时高峰，不能抵消长期处理不足。**若持续生产速度大于消费速度，lag 会持续增长。需要结合接口 P95/P99、生产错误、消费耗时、已提交 lag、数据库等待及积压恢复速度做压测；不要靠无限起 goroutine 或无界内存 channel 堆请求。

### 7. 新增一个高并发异步接口，复用哪些位置

1. **定义事件**：在 `internal/events/<领域>/` 定义小而稳定的 DTO，复用 `EventMeta`，实现 `EventName()` / `EventKey()`；确定 key 的顺序范围和热点分布。
2. **接入 Handler / Service**：复用现有路由、鉴权、限流、请求体上限及 context。若只接受异步命令，参考点赞的 `KafkaEventBus.Publish()`；若先修改权威数据，在 Repository 的同一短事务内调用 `SaveTx()`。
3. **实现消费者**：在 `internal/consumer/<领域>/` 实现 `Handle()`，有批量需求再实现 `HandleBatch()`；业务持久化/去重完成才返回 `nil`，失败要向上传递。
4. **注册和配置**：在 `internal/infrastructure/kafka/hg_runtime.go` 接入 handler；若新增独立消费组，还要同步扩展 `internal/pkg/kafka/hg_config.go` 的配置结构、`internal/pkg/config/hg_kafka_config.go` 的校验及环境 YAML，并准备 Topic/DLQ。已有消费组新增事件则扩展其事件处理逻辑。
5. **定义客户端如何判断完成**：异步接受可返回 `202`；若业务需要按任务查询结果，应增加稳定任务 ID 和查询契约。当前点赞接口只能查询状态，没有通用“查某条 Kafka 消息完成情况”的 API。

业务 Service 通常复用 EventBus 即可；拉取、手动提交 offset、DLQ 和生命周期已经由公共层封装。

### 8. 怎么确认全链路跑完，以及如何结束服务

按这个顺序观察一次真实请求：

1. **HTTP**：是否 `202`？`401/403` 先查公共校验/认证；`429/503` 结合网关与发送日志判断是否被拒绝。
2. **发送端**：点赞看 Producer 结果；Outbox 链路看指定 EventID 对应记录是否从 `pending` 变为 `published`，是否 `dead`，以及 dispatcher 是否退出。
3. **Kafka**：确认 `mlc.domain.events` 中有对应事件，核对 `eventId`、key 和 payload；可启动 UI 查看：

   ```bash
   docker compose -f deployments/docker-compose.kafka.yml up -d kafka-ui
   ```

   浏览器访问 `http://localhost:18080`。

4. **消费组**：只读检查 Interaction 已提交位点及 lag：

   ```bash
   docker exec mlc-kafka-1 /opt/kafka/bin/kafka-consumer-groups.sh \
     --bootstrap-server kafka-1:9092,kafka-2:9092,kafka-3:9092 \
     --describe --group mlc-go-debug-interaction
   ```

5. **MySQL**：按 EventID 定位 `video_interaction_inbox`，按用户/投稿/操作定位 `video_user_interactions`，检查对应计数分片；相关查询应走索引并限制范围。
6. **读侧与监控**：再看 `/state` 的 Redis 结果；默认管理地址为 `http://127.0.0.1:9091/readyz` 和 `/metrics`。关注 `mlc_kafka_produce_failures_total`、`mlc_kafka_commit_failures_total`、`mlc_kafka_dlq_failures_total`；应用 lag 与 broker 已提交 lag 需分清，`readyz` 正常也不代表某条业务已完成。

**正常完成**是业务结果已持久化、对应消费组位点已推进；若还要求页面最终视图一致，再确认 Redis 投影。仅看到 Outbox `published`、接口 `202` 或 lag 降低，都不能单独代替业务结果核对；坏消息进入 DLQ 后位点也可能推进。

停止前台服务可按 `Ctrl+C`：`MLCApplication.Serve()` 响应退出信号并关闭 HTTP，随后 `MLCApplication.Close()` 释放后台资源；Kafka 关闭回调先停 Dispatcher、再停 Consumer，最后 `HGCloseKafka()` 尝试在 10 秒内 Flush 并关闭 Producer。它不会等待整个 Kafka Topic 积压清空；未完成事件依靠已提交 offset、Outbox 租约和业务幂等在后续恢复。

**最后记住这条主线：接请求 → 校验/限流 → 可靠接受命令 → Kafka 分区排队 → 消费者幂等落库 → 提交 offset → 查询业务结果。**

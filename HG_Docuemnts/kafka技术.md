
- [kafka中Topic、Partition、Offset解读](#kafka中Topic、Partition、Offset解读)

***
<br/><br/><br/>
># <h2 id="kafka中Topic、Partition、Offset解读">kafka中Topic、Partition、Offset解读</h2>



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
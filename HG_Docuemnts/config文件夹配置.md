- [**多实例kafka业务配置文件**](#多实例kafka业务配置文件)



<br/>

***
<br/><br/><br/>
># <h1 id="多实例kafka业务配置文件">多实例kafka业务配置文件</h1>

### **`/config/debug/kafka.yaml`**配置

# kafka.yaml 配置完整解读
这是一套**多实例 kafka 业务配置文件**，基于 kgo（go‑kafka客户端），区分 `business` 业务事件总线、`log` 日志总线；
里面定义：broker地址、重试次数、clientId、topic列表、**多个消费组(consumer)**开关、group_id。
> 对应你前面整套代码：
> - `mlc.domain.events`：领域事件主topic，`KafkaEventBus.Publish()` 往这里发事件；
> - feed / statistic / interaction / search / audit / danmaku：就是不同消费业务，对应不同的 `Consumer.Handle` 消费逻辑。

整体结构
```yaml
kafka:
  business: # 业务事件总线（核心，领域事件DomainEvent都发这里）
    brokers: # kafka broker集群地址
    retry: 3 # 发送重试次数
    client_id: mlc-go-debug-business # 生产者客户端标识
    topics: # 生产者要使用的topic
    consumers: # 多个消费组配置，每个消费组独立开关、group_id、client_id
      feed: # Feed流消费（已开启）
      search: # ES索引消费（关闭）
      statistic: # 统计消费，就是你前面看的统计Consumer（关闭待灰度）
      audit: # 审核消费（禁止开启）
      interaction: # 互动事件消费（开启）
      danmaku: # 弹幕历史消费（关闭）
  log: # 独立日志kafka总线，专门打日志事件，和业务隔离
    brokers:
    retry:1
    client_id: mlc-go-debug-log
```

---

## 一、kafka.business 业务事件总线（最重要）
```yaml
kafka:
  business:
    # 本地 Compose 暴露到宿主机的 seed broker；容器部署时必须替换为可路由地址。
    brokers:
      - localhost:19092
      - localhost:29092
      - localhost:39092
    retry: 3
    client_id: mlc-go-debug-business
    topics:
      - mlc.domain.events
```

1. **brokers**
kafka集群种子节点列表。
> 注释：`本地 Compose 暴露到宿主机的 seed broker；容器部署时必须替换为可路由地址。`
- 本地开发：本机访问 `localhost:19092`；
- **容器内部跑服务时不能用 localhost**，localhost是容器自己，要填写kafka容器的service名称，例如 `kafka‑1:9092`。
> kgo只需要传入部分broker，内部会自动发现集群全部broker。

2. `retry:3`
**生产者发送消息失败重试次数**。调用 `ProduceSync`/`ProduceAsync` 发送失败，最多自动重试3次，再返回错误。

3. `client_id: mlc-go-debug-business`
kafka协议里的客户端标识；kafka broker日志、监控指标可以看到这个client‑id，用于排查是哪个客户端发出流量。`‑debug` 代表这是开发环境配置。

4. `topics: [ mlc.domain.events ]`
业务领域事件主Topic。
前面 `KafkaEventBus.Publish()` 发布所有领域事件，全部发送到这个topic。
> 视频发布、投稿、互动、审核等事件，全部往 `mlc.domain.events` 输出；
> 多个不同消费组，消费同一个topic，实现**一个事件，多组业务并行处理**。

> Kafka特性：多个消费组消费同一个topic，互相互不干扰。
> 例如 feed消费组、statistic统计消费组，都消费 `mlc.domain.events`，各自维护自己的offset。

---

## 二、business.consumers：各个消费组配置
> 每一个子项（feed / statistic / search / interaction …）代表**一个独立kafka消费组**。
每个消费组独立配置：
- `enabled`：开关，是否启动这个消费goroutine；
- `group_id`：kafka消费组ID，**决定offset存储在哪**；
- `client_id`：该消费者的客户端标识。

> ⚠️重要：`enabled: false`，程序不会启动该消费循环，不会连接kafka，不会拉取消息。

### 1. feed（enabled: true，已开启）
```yaml
feed:
# Feed v2 使用 64 分片 ZSET 和 Kafka partition offset 水位，避免全局热 key 与 EventID TTL 失效。
enabled: true
group_id: mlc-go-debug-feed
client_id: mlc-go-debug-feed
```
- 职责：**Feed流构建消费**，对应你之前看的Redis Feed投影逻辑。
- 注释解读：
> Feed v2 使用 64 分片 ZSET 和 Kafka partition offset 水位，避免全局热 key 与 EventID TTL 失效。

老版本Feed：用单一全局ZSet，大流量下变成热key；并且依赖EventID的Redis TTL做去重，TTL过期会丢失去重保护，会重复消费。
V2改进：
1. 拆成64个分片ZSET打散压力，消除全局热key；
2. 使用 **kafka partition offset水位** 做进度，不完全依赖Redis TTL去重。

> group_id：`mlc-go-debug‑feed`，feed消费组自己保存offset，和统计、搜索消费互不干扰。

### 2. search（enabled: false，关闭）
```yaml
search:
# 依赖 ES/OpenSearch mapping、批量客户端和失败补偿存储，契约完成前保持关闭。
enabled: false
group_id: mlc-go-debug-search
client_id: mlc-go-debug-search
```
消费领域事件，把数据同步到ES/OpenSearch，提供搜索能力。
> 注释：mapping、批量写入、失败补偿还没做完，不能开启，否则会出现ES写入异常、丢数据。

### 3. statistic（enabled: false，待灰度，就是你看的统计Consumer）
```yaml
statistic:
# 分片计数代码已完成；完成 Redis/ClickHouse 对账任务和验收告警后再单独灰度启用。
enabled: false
group_id: mlc-go-debug-statistic
client_id: mlc-go-debug-statistic
```
**对应你前面的 `Consumer.Handle()` 统计消费：**
消费 `mlc.domain.events`，做计数：Redis计数 + ClickHouse落库。
> 业务代码写完了，但是缺少：
> 1）Redis与ClickHouse对账任务；
> 2）异常告警；
> 没有上线验证，所以先关闭，灰度后再打开。

> 一旦打开，这个group就会开始消费topic，执行事件幂等、统计计数、写入ClickHouse。

### 4. audit（enabled: false，禁止启用）
```yaml
audit:
# 审核接口的超时、重试、幂等键和限流契约未完成，禁止启用。
enabled: false
group_id: mlc-go-debug-audit
client_id: mlc-go-debug-audit
```
审核业务消费，审核相关的限流、幂等、异常处理逻辑没做完，**禁止打开，打开会产生业务异常**。

### 5. interaction（enabled:true，已开启）
```yaml
interaction:
enabled: true
group_id: mlc-go-debug-interaction
client_id: mlc-go-debug-interaction
```
互动事件消费：点赞、收藏、评论这类互动领域事件处理。

### 6. danmaku（enabled: false，关闭）
```yaml
danmaku:
enabled: false
group_id: mlc-go-debug-danmaku-history
client_id: mlc-go-debug-danmaku-history
topics:
  - mlc.video.danmaku.created.v1
```
> 注意：这个消费组**不再消费公共的 mlc.domain.events**，单独消费弹幕专用topic `mlc.video.danmaku.created.v1`。
专门处理弹幕历史落库；当前关闭。

> 对比：feed / statistic / search / audit / interaction 默认继承上层 `topics: [mlc.domain.events]`；
> danmaku自己重写topics，订阅独立弹幕topic。

---

## 三、kafka.log 独立日志总线
```yaml
log:
  brokers:
    - localhost:19092
    - localhost:29092
    - localhost:39092
  retry: 1
  client_id: mlc-go-debug-log
```
独立的kafka生产者，专门输出日志、审计日志事件。
- 和业务事件总线 `business` 物理隔离；业务报错不会影响日志输出；
- `retry:1`，日志不做过多重试，日志允许极小概率丢失，不能阻塞主业务；
- 同样指向同一套kafka集群，只是逻辑分组隔离。

---

# 整体架构关系
1. 业务代码调用 `KafkaEventBus.Publish()` → 发送事件到 `mlc.domain.events`。
2. kafka topic `mlc.domain.events` 一份消息，多消费组并行消费：
    - ✅ feed消费组：构建用户feed流；
    - ✅ interaction消费组：处理点赞收藏互动；
    - ⏸ statistic消费组（关闭）：统计计数、ClickHouse存储；
    - ⏸ search消费组（关闭）：同步ES搜索；
    - ⏸ audit消费组（关闭）：审核处理。
3. 不同group_id，各自维护自己消费offset，互不干扰。
4. 弹幕走独立topic `mlc.video.danmaku.created.v1`。
5. 日志事件走独立的 `kafka.log` 生产者。

## 配置里的设计思想
1. **业务与日志流量隔离**：分开两套生产者配置，日志重试次数更低，日志不阻塞业务。
2. **消费组开关化配置**：通过 `enabled:true/false` 控制消费协程启停，**不需要改代码、不需要重新编译**，配置文件控制开启哪些业务消费。
> 新模块开发完，配置打开enabled即可上线，方便灰度。
3. 注释作为开发契约：明确标注哪些模块不能开启，前置依赖是什么，防止开发人员随意打开开关引发故障。
4. 开发环境命名规范：client_id / group_id 全部带 `-debug`，区分生产环境配置。

## 容易踩坑点
1. **容器部署 brokers 不能写 localhost**，要填集群内部可解析的hostname。
2. 每个消费组 `group_id` 不能重复；重复会导致多个实例属于同一个组，发生partition再平衡。
3. `statistic`、`audit` 等虽然代码存在，但是**enabled=false，不会跑**，不要误以为代码跑起来了。
4. 同一个topic多个消费组，每个group独立保存offset；statistic打开后会从头消费历史事件，需要注意数据压力。
5. danmaku消费组重写topics，不再消费mlc.domain.events，这点和其他消费组不一样。

## 和你之前代码串联
- `KafkaEventBus.Publish` → 发送到 `mlc.domain.events`；
- `statistic` 消费组一旦enabled=true，就会启动你看过的 `Consumer.Handle()`，做EventID幂等、Redis计数、ClickHouse入库；
- feed消费组开启，执行Feed流投影逻辑；
- 没开启的消费组，go程序完全不会启动对应的消费循环。

如果你需要，我可以画一份简单的mermaid文本流程图，展示：业务发布事件 → kafka topic → 各个消费组分别处理。

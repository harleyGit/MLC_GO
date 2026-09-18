<br/>

***
<br/><br/><br/>
># <h1 id="">clickhouse.yaml文件解读</h1>

## 路径：`config/base/clickhouse.yaml`

这段 `clickhouse.yaml` **不是单纯配置 ClickHouse 的连接信息**，它实际上包含了两块东西：

1. **ClickHouse 数据存储配置**：ClickHouse 要不要启用、怎么连接、用哪些表。
2. **Redis ↔ ClickHouse 数据一致性校验配置**：定期检查 Redis 里的统计数据和 ClickHouse 权威数据有没有“漂移”。

你可以把整个设计先理解成：

> **Redis 负责高并发、快速读写；ClickHouse 负责长期、权威的数据统计；reconcile 负责定期检查两边的数据有没有对不上。**

---

# 一、先看完整配置

你给的是：

```yaml
clickhouse:
  enabled: false
  scheme: http
  database: mlc
  user: default
  password: ""

  statistic_events_table: statistic_events
  statistic_totals_table: statistic_event_totals
  danmaku_history_table: video_danmaku_history

  write_timeout: 5s
  query_timeout: 15s

statistic:
  redis_generation: v2
  redis_shard_count: 64

# 首版只检测 Redis/ClickHouse 漂移并告警，不在线覆盖正在接收增量的 Redis。
reconcile_enabled: true
reconcile_interval: 5m
reconcile_timeout: 20s
```

可以把它画成：

```text
                    ┌──────────────────────┐
                    │       Go 服务         │
                    └──────────┬───────────┘
                               │
              ┌────────────────┴────────────────┐
              │                                 │
              ▼                                 ▼
       ┌─────────────┐                   ┌─────────────┐
       │    Redis    │                   │ ClickHouse  │
       │  高并发/快速 │                   │ 统计/持久化  │
       └──────┬──────┘                   └──────┬──────┘
              │                                 │
              │                                 │
              └───────────┐       ┌─────────────┘
                          ▼       ▼
                    ┌─────────────────┐
                    │   Reconciler    │
                    │ 一致性校验/对账  │
                    └────────┬────────┘
                             │
                             ▼
                       发现数据漂移
                             │
                             ▼
                           告警
```

下面逐个解释。

---

# 二、`clickhouse.enabled`

```yaml
clickhouse:
  enabled: false
```

这是最重要的开关之一。

意思是：

> **当前环境是否启用 ClickHouse。**

现在：

```yaml
enabled: false
```

表示：

```text
ClickHouse功能关闭
```

例如开发环境可能没有部署 ClickHouse：

```yaml
clickhouse:
  enabled: false
```

生产环境：

```yaml
clickhouse:
  enabled: true
```

那么程序才会真正初始化 ClickHouse。

---

## 为什么需要这个开关？

因为你的 Go 服务可能支持不同部署模式。

### 开发环境

```text
Go
 │
 ├── Redis
 │
 └── MySQL
```

没有 ClickHouse。

### 生产环境

```text
Go
 │
 ├── Redis
 ├── MySQL
 └── ClickHouse
```

所以程序不能写死：

```go
clickhouse.Connect(...)
```

而应该：

```go
if cfg.ClickHouse.Enabled {
    // 初始化 ClickHouse
}
```

这样开发环境就可以不依赖 ClickHouse。

---

# 三、`scheme: http`

```yaml
scheme: http
```

表示 ClickHouse 使用什么协议连接。

这里：

```text
http
```

通常表示通过 ClickHouse HTTP 接口访问。

例如：

```text
http://clickhouse:8123
```

ClickHouse 常见端口：

```text
8123  HTTP
9000  Native TCP
```

所以：

```yaml
scheme: http
```

一般对应：

```text
http://xxx:8123
```

---

# 四、`database: mlc`

```yaml
database: mlc
```

表示使用 ClickHouse 中的哪个数据库。

例如 ClickHouse：

```text
ClickHouse
    │
    ├── system
    ├── default
    └── mlc
         ├── statistic_events
         ├── statistic_event_totals
         └── video_danmaku_history
```

你的 Go 服务主要使用：

```text
mlc
```

这个 database。

---

# 五、`user` 和 `password`

```yaml
user: default
password: ""
```

表示 ClickHouse 登录账号。

这里：

```text
用户名 = default
密码 = 空
```

也就是：

```text
default / ""
```

开发环境可能这么配置。

生产环境一般不建议这样：

```yaml
user: default
password: ""
```

而应该使用独立账号和密码，例如：

```yaml
user: statistic_service
password: xxxxx
```

---

# 六、三个 `table` 是干什么的？

这是这份配置里面非常关键的一部分。

```yaml
statistic_events_table: statistic_events

statistic_totals_table: statistic_event_totals

danmaku_history_table: video_danmaku_history
```

它们实际上对应三个不同用途的数据表。

---

# 七、`statistic_events_table`

```yaml
statistic_events_table: statistic_events
```

表示：

> **统计事件明细表。**

例如你的系统里面有：

```text
点赞
投币
收藏
分享
播放
弹幕
```

那么可能产生大量事件：

```text
video 1001 点赞
video 1001 点赞
video 1001 投币
video 1002 播放
video 1001 分享
...
```

这些原始事件可以写到：

```text
statistic_events
```

例如：

```text
┌────────────┬──────────┬────────┬─────────────┐
│ event_id   │ video_id │ type   │ created_at  │
├────────────┼──────────┼────────┼─────────────┤
│ 100001     │ 1001     │ like   │ 10:01:01    │
│ 100002     │ 1001     │ like   │ 10:01:03    │
│ 100003     │ 1001     │ coin   │ 10:01:05    │
│ 100004     │ 1002     │ like   │ 10:01:08    │
└────────────┴──────────┴────────┴─────────────┘
```

这种表数据量可能非常大。

所以 ClickHouse 很适合。

---

# 八、`statistic_totals_table`

```yaml
statistic_totals_table: statistic_event_totals
```

这个和上面的不一样。

它更像：

> **聚合后的统计结果。**

例如原始事件：

```text
video 1001
    like
    like
    like
    coin
    coin
    share
```

经过统计以后：

```text
video_id = 1001

like  = 3
coin  = 2
share = 1
```

存到：

```text
statistic_event_totals
```

可能类似：

```text
┌──────────┬──────┬──────┬───────┐
│ video_id │ like │ coin │ share │
├──────────┼──────┼──────┼───────┤
│ 1001     │ 3    │ 2    │ 1     │
│ 1002     │ 8    │ 5    │ 3     │
└──────────┴──────┴──────┴───────┘
```

所以：

```text
statistic_events
```

更偏：

> **事件明细**

而：

```text
statistic_event_totals
```

更偏：

> **聚合结果**

---

# 九、`danmaku_history_table`

```yaml
danmaku_history_table: video_danmaku_history
```

这个就比较明确：

> **弹幕历史数据。**

例如：

```text
video_id = 1001

用户A：这个视频太好笑了
用户B：哈哈哈哈
用户C：学到了
```

长期历史数据可以进入：

```text
video_danmaku_history
```

ClickHouse 非常适合这种：

```text
大量写入
大量历史数据
时间范围查询
统计分析
```

的场景。

---

# 十、`write_timeout: 5s`

```yaml
write_timeout: 5s
```

表示：

> **写 ClickHouse 最多允许等待 5 秒。**

例如 Go：

```go
ctx, cancel := context.WithTimeout(
    context.Background(),
    5*time.Second,
)
defer cancel()

err := clickhouse.Write(ctx, data)
```

如果 ClickHouse 一直不响应：

```text
0s
1s
2s
3s
4s
5s
```

超过 5 秒：

```text
timeout
```

程序就不继续无限等待。

---

# 十一、为什么一定要有 timeout？

因为后端服务不能无限等待。

假设：

```text
Go
 │
 │ 写 ClickHouse
 ▼
ClickHouse
 │
 │ 卡住
 ▼
一直没有响应
```

如果没有 timeout：

```text
goroutine
    ↓
一直等待
    ↓
越来越多
    ↓
资源耗尽
```

所以：

```yaml
write_timeout: 5s
```

实际上属于：

> **故障隔离 / 超时控制**

---

# 十二、`query_timeout: 15s`

```yaml
query_timeout: 15s
```

和刚才的：

```yaml
write_timeout
```

对应。

区别是：

```text
write_timeout
    ↓
写 ClickHouse

query_timeout
    ↓
查询 ClickHouse
```

例如 Reconciler 要查询：

```sql
SELECT ...
FROM statistic_event_totals
WHERE video_id = ?
```

最多允许：

```text
15 秒
```

---

# 十三、为什么查询 timeout 比写 timeout 长？

这里：

```text
写：5s
查询：15s
```

是一个比较合理的工程配置。

因为：

### 写入

通常希望：

```text
快速写入
```

所以：

```text
5s
```

### 查询

特别是统计查询：

```sql
GROUP BY
SUM
COUNT
时间范围
大量数据扫描
```

可能需要更多时间。

所以：

```text
15s
```

---

# 十四、再看 `statistic`

```yaml
statistic:
  redis_generation: v2
  redis_shard_count: 64
```

这一部分非常重要。

它说明：

> **统计数据在 Redis 中采用什么版本、多少分片。**

---

# 十五、`redis_generation: v2`

```yaml
redis_generation: v2
```

可以理解成：

> **Redis 统计数据结构/命名规则的版本号。**

比如第一版可能：

```text
v1
```

Redis Key：

```text
statistic:video:1001
```

后来升级到：

```text
v2
```

可能变成：

```text
statistic:v2:shard:12:video:1001
```

这样做的好处是：

> **升级 Redis 数据结构的时候，可以明确区分不同版本。**

例如：

```text
v1
 │
 ├── 老 Redis Key
 │
 └── 老数据结构

v2
 │
 ├── 新 Redis Key
 │
 └── 新数据结构
```

---

# 十六、为什么 Redis 要分 64 个 shard？

```yaml
redis_shard_count: 64
```

表示：

> **统计 Redis 数据逻辑上分成 64 个分片。**

这和你之前问过的：

```go
func GetFeedShard(submissionID string, shardCount int) int
```

这种思想是类似的。

比如：

```go
shard := GetStatisticShard(videoID, 64)
```

可能得到：

```text
video 1001 → shard 12
video 1002 → shard 37
video 1003 → shard 5
```

于是：

```text
Redis Statistic

Shard 0
Shard 1
Shard 2
...
Shard 63
```

总共：

```text
64 shards
```

---

# 十七、为什么要分片？

核心就是：

> **避免所有统计数据集中在一个 Redis Key / 一个 Redis 数据结构 / 一个热点节点上。**

假设有：

```text
1000 万视频
```

如果所有统计都集中：

```text
Redis
 │
 └── statistic
       │
       └── 巨大 Hash
```

可能产生：

```text
热点
内存压力
单 Key 压力
并发竞争
```

分成 64 个逻辑 shard：

```text
               Redis Statistic
                      │
       ┌──────┬───────┼───────┬──────┐
       ▼      ▼       ▼       ▼      ▼
      S0     S1      S2      ...     S63
```

数据被分散。

---

# 十八、重点来了：`reconcile`

下面是整个配置里**最值得理解的部分**：

```yaml
reconcile_enabled: true
reconcile_interval: 5m
reconcile_timeout: 20s
```

它解决的是：

> **Redis 和 ClickHouse 数据不一致怎么办？**

---

# 十九、为什么 Redis 和 ClickHouse 会不一致？

假设系统收到一个：

```text
视频 1001 点赞
```

业务链路可能是：

```text
客户端
   ↓
Go API
   ↓
Redis
   ↓
Kafka
   ↓
ClickHouse
```

假设 Redis 已经：

```text
like = 100
```

但是 ClickHouse 因为：

```text
Kafka 延迟
消费者异常
ClickHouse 写入失败
网络异常
```

可能只有：

```text
like = 98
```

于是：

```text
Redis      = 100
ClickHouse = 98
```

这就是：

> **数据漂移（drift）**

---

# 二十、为什么会设计 Reconciler？

就是定期做：

```text
Redis
  ↓
100

ClickHouse
  ↓
98

      ↓

发现差异

      ↓

告警
```

也就是：

> **对账 / 一致性校验**

你之前问过的 `HGReconciler`：

```go
type HGReconciler struct {
    authority AggregateReader
    redis     RedisHashReader
    config    HGReconcileConfig
}
```

就是干这个事情的。

---

# 二十一、`reconcile_enabled: true`

```yaml
reconcile_enabled: true
```

意思：

> **开启 Redis / ClickHouse 一致性检查。**

如果：

```yaml
reconcile_enabled: false
```

那么：

```text
Redis
   ↓
不检查
   ↓
ClickHouse
```

即使出现：

```text
Redis = 100
ClickHouse = 80
```

也不会进行 Reconcile。

---

# 二十二、`reconcile_interval: 5m`

```yaml
reconcile_interval: 5m
```

表示：

> **每 5 分钟执行一次对账。**

类似：

```go
ticker := time.NewTicker(5 * time.Minute)
```

然后：

```text
第一次
   ↓
执行 Reconcile

5分钟
   ↓
执行 Reconcile

5分钟
   ↓
执行 Reconcile

5分钟
   ↓
执行 Reconcile
```

---

# 二十三、`reconcile_timeout: 20s`

```yaml
reconcile_timeout: 20s
```

表示：

> **一次完整的 Reconcile 最多执行 20 秒。**

例如：

```go
reconcileCtx, cancel := context.WithTimeout(
    ctx,
    20*time.Second,
)
defer cancel()

r.Reconcile(reconcileCtx)
```

如果 20 秒还没完成：

```text
context deadline exceeded
```

这次对账终止。

---

# 二十四、为什么 Reconcile 不能无限跑？

假设：

```text
Reconcile
    ↓
查询 Redis
    ↓
查询 ClickHouse
    ↓
GROUP BY
    ↓
数据很多
```

如果 ClickHouse 突然非常慢：

```text
1分钟
2分钟
3分钟
...
```

而：

```text
reconcile_interval = 5m
```

就可能出现：

```text
Reconcile 1
───────────────────────────────→
                 10分钟

Reconcile 2
          ──────────────────────→
```

甚至多个 Reconcile 同时跑。

所以：

```yaml
reconcile_timeout: 20s
```

实际上是：

> **防止对账任务拖死服务。**

---

# 二十五、最关键的注释是什么意思？

你这里有一句：

```yaml
# 首版只检测 Redis/ClickHouse 漂移并告警，不在线覆盖正在接收增量的 Redis。
```

这句话非常重要。

它说明目前的设计是：

> **发现问题，但是不自动修 Redis。**

例如：

```text
Redis = 100
ClickHouse = 98
```

Reconciler：

```text
发现：
Redis ≠ ClickHouse

      ↓

记录日志

      ↓

Prometheus Metric

      ↓

告警

      ↓

人工处理
```

而不是：

```text
Redis = 100
ClickHouse = 98

      ↓

自动修改 Redis

Redis = 98
```

---

# 二十六、为什么不直接自动覆盖 Redis？

因为 Redis **可能正在接收实时增量**。

例如：

```text
当前 Redis = 100
```

与此同时：

```text
用户又点赞 1 次
```

业务代码：

```text
Redis
100 → 101
```

这时候 Reconciler 查询到：

```text
ClickHouse = 98
Redis = 100
```

它觉得：

```text
Redis 错了
应该改成 98
```

于是：

```text
Redis
100 → 98
```

但业务线程刚刚已经：

```text
100 → 101
```

结果可能变成：

```text
101
 ↓
98
```

**刚收到的增量被覆盖了。**

这就是所谓：

> **在线覆盖正在接收增量的 Redis**

风险很大。

---

# 二十七、所以现在采用的是“只检测、不修复”

这是一种非常保守的第一阶段设计：

```text
                 ┌─────────────┐
                 │ Reconciler  │
                 └──────┬──────┘
                        │
                定期读取两边
                        │
          ┌─────────────┴─────────────┐
          ▼                           ▼
       Redis                       ClickHouse
       100                            98
          │                           │
          └─────────────┬─────────────┘
                        ▼
                     不一致
                        │
                        ▼
                       告警
```

而不是：

```text
不一致
  ↓
自动修复 Redis
```

这其实是一个比较合理的**第一版（首版）策略**。

---

# 二十八、把这套配置串起来

现在可以把整个配置理解成：

```text
                    Go 服务
                       │
          ┌────────────┴────────────┐
          │                         │
          ▼                         ▼
       Redis                    ClickHouse
          │                         │
          │                         │
          │                    mlc database
          │                         │
          │              ┌──────────┼──────────┐
          │              │          │          │
          │              ▼          ▼          ▼
          │         statistic_events
          │         statistic_event_totals
          │         video_danmaku_history
          │
          │
          ▼
    64 个逻辑 shard
          │
          │
          └──────────────┐
                         ▼
                    Reconciler
                         │
                  每 5 分钟一次
                         │
                  最多运行 20 秒
                         │
                         ▼
              Redis vs ClickHouse
                         │
                  ┌──────┴──────┐
                  │             │
                一致           不一致
                  │             │
                  ▼             ▼
                 OK            告警
                                │
                                ×
                         暂时不自动修复
```

---

# 二十九、这些配置可以分成 3 个层次

你以后看 Go 项目的 YAML，建议按这个思路理解。

## 第一层：基础设施连接配置

```yaml
clickhouse:
  enabled: false
  scheme: http
  database: mlc
  user: default
  password: ""
```

解决：

> **ClickHouse 在哪里？怎么连接？**

---

## 第二层：数据模型配置

```yaml
statistic_events_table: statistic_events
statistic_totals_table: statistic_event_totals
danmaku_history_table: video_danmaku_history
```

解决：

> **连接之后，数据放在哪里？**

---

## 第三层：可靠性 / 一致性配置

```yaml
write_timeout: 5s
query_timeout: 15s

redis_generation: v2
redis_shard_count: 64

reconcile_enabled: true
reconcile_interval: 5m
reconcile_timeout: 20s
```

解决：

> **服务怎么稳定运行？Redis 和 ClickHouse 数据怎么保证不会悄悄出现问题？**

---

# 三十、结合你这个 Go 工程，整个设计实际上很像这样

你前面看到的：

```go
HGReconciler
```

以及：

```go
reconcileCtx, cancel := context.WithTimeout(
    ctx,
    r.config.Timeout,
)
```

现在就可以完全对应起来：

```yaml
reconcile_timeout: 20s
```

↓

Go：

```go
reconcileCtx, cancel := context.WithTimeout(
    ctx,
    r.config.Timeout,
)
```

---

而：

```yaml
reconcile_interval: 5m
```

对应：

```go
ticker := time.NewTicker(
    r.config.ReconcileInterval,
)
```

---

而：

```yaml
redis_shard_count: 64
```

对应：

```go
GetStatisticShard(..., 64)
```

---

而：

```yaml
statistic_totals_table: statistic_event_totals
```

对应：

```go
SELECT ...
FROM statistic_event_totals
```

---

# 三十一、最终一句话理解

这份配置本质上是在定义一个：

> **“Redis 做实时高并发统计 + ClickHouse 做权威统计存储 + Reconciler 定期对账”的数据架构。**

其中：

| 配置                       | 作用                       |
| ------------------------ | ------------------------ |
| `clickhouse.enabled`     | 是否启用 ClickHouse          |
| `scheme`                 | ClickHouse 连接协议          |
| `database`               | ClickHouse 数据库           |
| `user/password`          | ClickHouse 登录信息          |
| `statistic_events_table` | 统计事件明细表                  |
| `statistic_totals_table` | 统计聚合结果表                  |
| `danmaku_history_table`  | 弹幕历史表                    |
| `write_timeout`          | ClickHouse 写入最长等待时间      |
| `query_timeout`          | ClickHouse 查询最长等待时间      |
| `redis_generation`       | Redis 统计数据结构版本           |
| `redis_shard_count`      | Redis 统计逻辑分片数量           |
| `reconcile_enabled`      | 是否开启 Redis/ClickHouse 对账 |
| `reconcile_interval`     | 多久对账一次                   |
| `reconcile_timeout`      | 一次对账最多执行多久               |

**尤其要记住这三个角色：**

```text
Redis
= 快、实时、承受高并发

ClickHouse
= 海量统计、历史数据、权威累计值

Reconciler
= 不负责正常业务写入
= 专门负责“查两边是否对得上”
= 首版发现问题后告警，不直接覆盖 Redis
```

这也正好解释了你之前看到的 `HGReconciler` 为什么有 `authority` 和 `redis` 两个 Reader：**它本质上就是一个“数据对账员”——Redis 是实时投影，ClickHouse 是权威累计值，然后定期检查投影有没有偏离权威值。**

- [docker-compose.kafka.yml 文件解析](#docker-compose-kafka-yml文件解析)
	- [组件清单](#组件清单)
	- [Kafka KRaft 三节点集群](#Kafka-KRaft三节点集群)
	- [配套组件](#配套组件)
	- [数据卷与密钥](#数据卷与密钥)
	- [本地开发链路](#本地开发链路)
	- [开发环境注意事项](#开发环境注意事项)
	- [常见问题](#常见问题)

***
<br/><br/><br/>

> <h1 id="docker-compose-kafka-yml文件解析">docker-compose.kafka.yml 文件解析</h1>

**用途：** 在本地开发环境一键拉起 MLC 项目依赖栈，包括 Kafka KRaft 三节点集群、ClickHouse、Redis Statistic、Prometheus、Grafana、Alertmanager、Kafka UI 和 Kafka Offset Exporter。

> **注意：该配置仅用于开发环境，不能直接用于生产。**

***
<br/>

> <h2 id="组件清单">组件清单</h2>

1. kafka‑1 / kafka‑2 / kafka‑3：KRaft 3 节点 Kafka 集群（无 ZooKeeper）
2. kafka‑ui：Web 界面看 Kafka 集群、topic、消费组、消息
3. kafka‑committed‑offset‑exporter：把 kafka 消费 offset 暴露成 prometheus 指标，看消费 lag
4. clickhouse：统计模块存储（statistic 消费模块落数）
5. redis‑statistic：统计模块专用 Redis，和本机 6379 隔离，用于 feed 分片 ZSET 计数
6. prometheus：时序指标采集
7. alertmanager：告警转发（webhook 推告警）
8. grafana：大盘可视化面板

***
<br/>

> <h2 id="Kafka-KRaft三节点集群">Kafka KRaft 三节点集群</h2>

版本：`apache/kafka:3.7.1`，**KRaft 模式，抛弃 Zookeeper**，每个节点同时是 `broker + controller`。

### 关键环境变量

1. `CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk`：整个集群的唯一 ID，**三个节点必须完全相同**。

2. `KAFKA_NODE_ID: 1 / 2 / 3`：每个节点的唯一编号，不能重复。

3. `KAFKA_PROCESS_ROLES: broker,controller`

- broker：负责接收生产、消费消息；
- controller：集群元数据管理（选主、分区分配、topic 管理）；
  三节点都承担两种角色，controller quorum 三选二达成共识。

4. `KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka‑1:9093,2@kafka‑2:9093,3@kafka‑3:9093`
   KRaft 控制器仲裁配置，**9093 端口专门给 controller 内部元数据通信，业务客户端不会连这个端口**。

5. `KAFKA_LISTENERS` / `KAFKA_ADVERTISED_LISTENERS`（非常容易踩坑）

```yaml
KAFKA_LISTENERS: INTERNAL://:9092,EXTERNAL://:19092,CONTROLLER://:9093
KAFKA_ADVERTISED_LISTENERS: INTERNAL://kafka‑1:9092,EXTERNAL://localhost:19092
```

- `INTERNAL://kafka‑1:9092`：**compose 容器网络内部访问**，其他容器（kafka‑ui/exporter）用这个地址；
- `EXTERNAL://localhost:19092`：**你的本机 Go 应用（不在 docker 容器里）访问**，bootstrap 写 `localhost:19092,localhost:29092,localhost:39092`；
  > 原理：Kafka 客户端拿到元数据，broker 返回`advertised`地址，客户端就用这个地址建立真实连接。配错就会出现：能连上 bootstrap，但是元数据返回容器主机名，本机 Go 程序无法连接。

6. `KAFKA_AUTO_CREATE_TOPICS_ENABLE: "false"`：**关闭自动创建 Topic**。本地需要手动执行 `make kafka-init` 创建 Topic；生产环境应由 IaC 或发布系统统一管理，避免因业务代码拼写错误生成无效 Topic。

7. 内部 topic 副本配置

```yaml
KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 3
KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR:3
KAFKA_TRANSACTION_STATE_LOG_MIN_ISR:2
```

- `__consumer_offsets`：保存消费组 offset 位点，副本 3；单节点挂掉 offset 不丢；
- 事务状态 topic，为 outbox 事务消息做准备；min.isr=2，三节点集群允许挂掉 1 个节点集群仍然可用。

8. volumes 数据卷
   `kafka_1_data/kafka_2_data/kafka_3_data`
   持久化 kafka 磁盘数据；删除 volume，所有 topic、消息、消费 offset 全部清空。

**Bootstrap 地址：**

- 本机 Go 服务：`localhost:19092,localhost:29092,localhost:39092`
- Compose 内部容器：`kafka-1:9092,kafka-2:9092,kafka-3:9092`

***
<br/>

> <h2 id="配套组件">配套组件</h2>

### Kafka UI

浏览器访问：`http://localhost:18080`

- 可视化看 topic、分区、消息内容、消费组 group、offset、lag；
- compose 网络内，bootstrap 填 `kafka‑1:9092,kafka‑2:9092,kafka‑3:9092`；
  > 开发调试神器，替代命令行 kafka‑console‑consumer。

### Kafka Committed Offset Exporter

`danielqsj/kafka‑exporter`
作用：**读取 kafka 各个消费组提交 offset，输出 prometheus 指标**。
指标例子：`kafka_consumergroup_current_offset`、`kafka_consumergroup_lag`。

> 对应你 yaml 里面 4 个消费组 feed/search/statistic/audit，这个 exporter 会采集他们的 lag，prometheus 抓取，grafana 画图，lag 过高触发 alertmanager 告警。
> 注意：注释写：低频抓取，避免对 kafka 管理接口造成压力。

### ClickHouse

`clickhouse/clickhouse‑server:24.8.14.39`
端口映射：`18123:8123`，本地 go 连接 `localhost:18123`

- 初始化脚本挂载：`./clickhouse/001_statistic_events.sql`，容器启动自动建表；
- 数据库 `mlc`，用户`default`，无密码；
- 对应项目中 `statistic` consumer：消费 kafka 消息做统计，写入 clickhouse。

### Redis Statistic

`redis:7.4.2‑alpine`
端口：`127.0.0.1:16379:6379`

> 和本机你电脑上默认 6379redis 做隔离，专门给 feed、statistic 模块用。

- AOF 持久化开启；`noeviction`：禁止淘汰 key，业务数据不能被 redis 驱逐；
- 对应代码：Feed v2 64 分片 ZSET 就存在这个 redis。

### Prometheus

浏览器访问 `http://localhost:19090`

- 时序数据库，采集：kafka‑exporter 指标、Go 服务埋点 metrics；
- `extra_hosts: host.docker.internal:host‑gateway`：docker 容器内部可以访问**宿主机上运行的 Go 服务**，抓取 Go 应用暴露的 metrics。
- 挂载本地 prometheus.yml、告警 rules 规则文件。

### Alertmanager

端口 `19093`
接收 prometheus 告警，转发 webhook；密钥从文件读取`ALERTMANAGER_WEBHOOK_URL_FILE`，不硬编码在 yml。告警可以推到钉钉/企业微信。

### Grafana

`http://localhost:13000`，账号 admin/admin
可视化大盘，预配置 provisioning，自动加载 dashboard，看 kafka lag、clickhouse、redis、go 服务指标。

***
<br/>

> <h2 id="数据卷与密钥">数据卷与密钥</h2>

### Volumes

全部命名卷：持久化各个组件数据。

> 调试的时候，想重置整个环境：`docker compose down -v`，会把所有 volume 全部删除，kafka 消息、offset、clickhouse 表数据全部清空。

### Secrets

`alertmanager_webhook_url`：把 webhook 地址放到外部文件，避免 yml 明文写密钥。

***
<br/>

> <h2 id="本地开发链路">本地开发链路</h2>

1. 执行 `docker compose up` 启动整套依赖。
2. 执行 `make kafka-init` 创建业务 Topic。
3. 启动本地 Go 服务，读取 `kafka.yaml`：
   - feed consumer 开启，消费 topic，写入 redis‑statistic 分片 ZSET；
   - search/statistic/audit：enabled=false 不启动；
4. kafka‑exporter 采集 feed 消费组 offset、lag；
5. prometheus 拉取 exporter + Go 服务 metrics；
6. grafana 看大盘；lag 高了 prometheus 触发告警，alertmanager 推送通知；
7. kafka‑ui 浏览器查看 topic 消息、group 消费位点。

> 后续开发：当要调试 statistic 模块，把 yaml 中 `statistic.enabled:true`，重启 go 服务，statistic consumer 启动，消费 kafka 消息写入 clickhouse。

***
<br/>

> <h2 id="开发环境注意事项">开发环境注意事项</h2>

1. **不能直接用于生产**：缺少鉴权、TLS 加密、磁盘规划和生产告警。
2. Kafka 已关闭自动创建 Topic，业务 Topic 必须预先初始化。
3. 本机 Go 程序使用 `localhost:19092,localhost:29092,localhost:39092`；容器内部组件使用 `kafka-1:9092,kafka-2:9092,kafka-3:9092`。
4. 删除 Volume 会丢失 Kafka 消息、消费 Offset 和 ClickHouse 数据。
5. Redis Statistic 端口绑定到 `127.0.0.1`，仅限本机开发访问。

***
<br/>

> <h2 id="常见问题">常见问题</h2>

1. 未区分 `INTERNAL` / `EXTERNAL` Listener，导致本机 Go 程序获取到容器主机名后无法连接。
2. 未执行 `make kafka-init`，Topic 不存在，Consumer 启动后报错。
3. 重启 Compose 时未使用 `-v`，Volume 会继续保留，Kafka 原有 Offset 仍然存在。
4. Prometheus 无法抓取宿主机 Go 服务 Metrics：macOS 和 Windows 通常内置 `host.docker.internal`，部分 Linux 环境需要配置 `extra_hosts`。

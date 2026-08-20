# docker‑compose.yml 完整解读

> 用途：**本地开发环境一键拉起整套 MLC 项目依赖栈**，完全对应你前面 Go 服务代码：Kafka(KRaft 三节点)、ClickHouse、Redis‑statistic、监控全套(prometheus/grafana/alertmanager)、kafka‑ui、kafka offset exporter。
> 注意：注释明确写死：**这只是开发环境，不能直接上生产**。

整体组件清单：

1. kafka‑1 / kafka‑2 / kafka‑3：KRaft 3 节点 Kafka 集群（无 ZooKeeper）
2. kafka‑ui：Web 界面看 Kafka 集群、topic、消费组、消息
3. kafka‑committed‑offset‑exporter：把 kafka 消费 offset 暴露成 prometheus 指标，看消费 lag
4. clickhouse：统计模块存储（statistic 消费模块落数）
5. redis‑statistic：统计模块专用 Redis，和本机 6379 隔离，用于 feed 分片 ZSET 计数
6. prometheus：时序指标采集
7. alertmanager：告警转发（webhook 推告警）
8. grafana：大盘可视化面板

---

## 1. Kafka‑1/2/3：KRaft 三节点集群重点拆解

版本：`apache/kafka:3.7.1`，**KRaft 模式，抛弃 Zookeeper**，每个节点同时是 `broker + controller`。

### 关键环境变量

1. `CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk`
   整个集群唯一 ID，**三个节点必须一模一样**，KRaft 集群靠这个识别属于同一个集群。

2. `KAFKA_NODE_ID: 1 /2 /3`
   每个节点唯一编号，不能重复。

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

6. `KAFKA_AUTO_CREATE_TOPICS_ENABLE: "false"`
   **关闭自动创建 topic**。
   本地不会发消息就自动生成 topic，需要手动执行 `make kafka‑init` 创建 topic；生产环境由 IaC/发布系统管理 topic。防止业务代码写错，随意生成乱七八糟 topic。

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

> 👉 本机 Go 服务连接 bootstrap：
> `localhost:19092,localhost:29092,localhost:39092`
> 👉 compose 内部容器连接 bootstrap：
> `kafka‑1:9092,kafka‑2:9092,kafka‑3:9092`

---

## 2. kafka‑ui

浏览器访问：`http://localhost:18080`

- 可视化看 topic、分区、消息内容、消费组 group、offset、lag；
- compose 网络内，bootstrap 填 `kafka‑1:9092,kafka‑2:9092,kafka‑3:9092`；
  > 开发调试神器，替代命令行 kafka‑console‑consumer。

## 3. kafka‑committed‑offset‑exporter

`danielqsj/kafka‑exporter`
作用：**读取 kafka 各个消费组提交 offset，输出 prometheus 指标**。
指标例子：`kafka_consumergroup_current_offset`、`kafka_consumergroup_lag`。

> 对应你 yaml 里面 4 个消费组 feed/search/statistic/audit，这个 exporter 会采集他们的 lag，prometheus 抓取，grafana 画图，lag 过高触发 alertmanager 告警。
> 注意：注释写：低频抓取，避免对 kafka 管理接口造成压力。

## 4. clickhouse

`clickhouse/clickhouse‑server:24.8.14.39`
端口映射：`18123:8123`，本地 go 连接 `localhost:18123`

- 初始化脚本挂载：`./clickhouse/001_statistic_events.sql`，容器启动自动建表；
- 数据库 `mlc`，用户`default`，无密码；
- 对应项目中 `statistic` consumer：消费 kafka 消息做统计，写入 clickhouse。

## 5. redis‑statistic

`redis:7.4.2‑alpine`
端口：`127.0.0.1:16379:6379`

> 和本机你电脑上默认 6379redis 做隔离，专门给 feed、statistic 模块用。

- AOF 持久化开启；`noeviction`：禁止淘汰 key，业务数据不能被 redis 驱逐；
- 对应代码：Feed v2 64 分片 ZSET 就存在这个 redis。

## 6. prometheus

浏览器访问 `http://localhost:19090`

- 时序数据库，采集：kafka‑exporter 指标、Go 服务埋点 metrics；
- `extra_hosts: host.docker.internal:host‑gateway`：docker 容器内部可以访问**宿主机上运行的 Go 服务**，抓取 Go 应用暴露的 metrics。
- 挂载本地 prometheus.yml、告警 rules 规则文件。

## 7. alertmanager

端口 `19093`
接收 prometheus 告警，转发 webhook；密钥从文件读取`ALERTMANAGER_WEBHOOK_URL_FILE`，不硬编码在 yml。告警可以推到钉钉/企业微信。

## 8. grafana

`http://localhost:13000`，账号 admin/admin
可视化大盘，预配置 provisioning，自动加载 dashboard，看 kafka lag、clickhouse、redis、go 服务指标。

## volumes 块

全部命名卷：持久化各个组件数据。

> 调试的时候，想重置整个环境：`docker compose down -v`，会把所有 volume 全部删除，kafka 消息、offset、clickhouse 表数据全部清空。

## secrets

`alertmanager_webhook_url`：把 webhook 地址放到外部文件，避免 yml 明文写密钥。

---

# 和你前面 Go 业务代码完整串联（整个链路）

1. docker compose up 启动整套依赖；
2. 执行 `make kafka‑init` 创建业务 topic；
3. 启动本地 Go 服务，读取 `kafka.yaml`：
   - feed consumer 开启，消费 topic，写入 redis‑statistic 分片 ZSET；
   - search/statistic/audit：enabled=false 不启动；
4. kafka‑exporter 采集 feed 消费组 offset、lag；
5. prometheus 拉取 exporter + Go 服务 metrics；
6. grafana 看大盘；lag 高了 prometheus 触发告警，alertmanager 推送通知；
7. kafka‑ui 浏览器查看 topic 消息、group 消费位点。

> 后续开发：当要调试 statistic 模块，把 yaml 中 `statistic.enabled:true`，重启 go 服务，statistic consumer 启动，消费 kafka 消息写入 clickhouse。

## 开发环境注意事项（注释重点）

1. ❗**不能直接用于生产**：缺少鉴权、TLS 加密、磁盘规划、生产告警；
2. kafka 关闭自动创建 topic，topic 必须预先初始化；
3. 本机 Go 程序连接 kafka 用 `localhost:19092,29092,39092`；容器内部组件用 kafka‑1:9092；
4. 删除 volume 会丢失 kafka 所有消息与 offset；
5. redis‑statistic 端口绑定 127.0.0.1，外部机器无法访问，仅限本机开发。

## 常见踩坑点

1. 忘记区分 `INTERNAL` / `EXTERNAL` listener，Go 程序连 kafka 拿到容器主机名无法连接；
2. 不执行`make kafka‑init`，topic 不存在，consumer 直接报错；
3. 重启 compose 不加 `-v`，volume 保留，kafka 旧 offset 还在；
4. prometheus 抓不到本机 Go 服务 metrics：`host.docker.internal` 在 windows/mac 自带，部分 linux 需要配置 extra_hosts。

如果你需要，我可以简单梳理一份本地开发操作步骤（启动、初始化 topic、重置环境、看 lag）。

- [Prometheus主配置文件](#Prometheus主配置文件)
- [Grafana的数据源预配置文件](#Grafana的数据源预配置文件)
- [ Prometheus 的告警规则文件](#Prometheus的告警规则文件)


<br/>

***
<br/><br/><br/>
># <h1 id="Prometheus主配置文件">Prometheus主配置文件</h1>

## 这份 monitoring/prometheus.yml 的作用

这是 Prometheus 的**主配置文件**，定义了"抓什么、多久抓一次、告警规则从哪加载、告警发给谁"。逐段拆解：

---

### 1. `global` — 全局默认节奏

```yaml
global:
  scrape_interval: 15s      # 每 15 秒抓一次所有 target 的 metrics
  evaluation_interval: 15s  # 每 15 秒评估一次告警规则（判断是否触发告警）
```

**作用**：两个周期保持一致（都是 15s），意味着告警表达式的评估粒度和数据采集粒度对齐，不会出现"规则评估时数据还没到"的情况。

- `scrape_interval`：Prometheus 主动去拉 `/metrics` 的频率
- `evaluation_interval`：Prometheus 重新计算 `rule_files` 里告警表达式的频率

---

### 2. `rule_files` — 加载告警规则

```yaml
rule_files:
  - /etc/prometheus/rules/*.yml
```

**作用**：从容器内 `/etc/prometheus/rules/` 目录加载所有 `.yml` 告警规则文件。这个目录对应 compose 里挂的 `./monitoring/rules`。

里面通常是类似这样的规则：
```yaml
groups:
  - name: mlc-go-alerts
    rules:
      - alert: MlcGoHighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.1
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "mlc-go 错误率超过 10%"
```

Prometheus 每 15s 跑一遍这些表达式，满足条件就生成告警，发给下面的 Alertmanager。

---

### 3. `alerting` — 告警转发目标

```yaml
alerting:
  alertmanagers:
    - static_configs:
        - targets: [alertmanager:9093]
```

**作用**：Prometheus 触发的告警（包括 firing 和 resolved）统一转发给同 Compose 网络里的 **Alertmanager** 服务，端口 9093。

- `alertmanager` 是容器名，在同一个 docker-compose 网络里可以直接用服务名解析
- Alertmanager 负责后续的**去重、分组、抑制、静默、路由通知**（发邮件/钉钉/飞书/Webhook 等）
- Prometheus 本身只负责"检测到异常并生成告警"，不负责通知

---

### 4. `scrape_configs` — 抓取目标（核心）

定义了两个抓取任务：

#### Job 1: `mlc-go` — 抓应用自身的 metrics

```yaml
- job_name: mlc-go
  metrics_path: /metrics
  static_configs:
    - targets: [host.docker.internal:9091]
      labels:
        service: mlc-go
        environment: local
        component: application
```

**作用**：抓取你的 Go 应用（mlc-go）暴露在 **9091 管理端口**上的 `/metrics`。

几个设计要点：
- **用 `host.docker.internal:9091` 而不是 `localhost:9091`**：因为 Prometheus 跑在容器里，容器内的 `localhost` 是容器自己，不是宿主机。compose 里的 `extra_hosts` 就是为了让这个域名能解析到宿主机
- **独立管理端口 9091**：业务端口（比如 8080）和 metrics 端口分开，避免 `/metrics` 暴露在公网入口，也避免业务流量和监控流量互相影响
- **附加 labels**：`service` / `environment` / `component` 这些标签会附加到所有抓到的指标上，方便 PromQL 聚合筛选和告警路由

#### Job 2: `kafka-committed-offset-exporter` — 抓 Kafka 消费延迟

```yaml
- job_name: kafka-committed-offset-exporter
  scrape_interval: 60s      # 覆盖全局 15s，改成 60s 抓一次
  scrape_timeout: 15s       # 单次抓取超时 15s
  static_configs:
    - targets: [kafka-committed-offset-exporter:9308]
      labels:
        service: mlc-kafka
        environment: local
        component: kafka-committed-offset
```

**作用**：抓取一个独立的 **Kafka committed offset exporter**（端口 9308），用来监控 Kafka 消费组的 **committed offset lag**（已提交位点与最新位点的差距，即消费延迟）。

设计要点：
- **低频抓取（60s）**：offset lag 变化相对慢，不需要 15s 那么高频，减少 exporter 和 Kafka 的压力
- **独立 exporter，不跟应用进程内指标混**：应用进程内可以记录"处理了多少条消息"，但 committed offset 是 Kafka broker 端维护的消费位点，需要单独的 exporter 去查 broker，两者数据源独立，便于交叉验证（比如应用说处理完了，但 committed offset 没跟上 → 可能是 commit 逻辑有问题）
- `kafka-committed-offset-exporter` 是同 Compose 网络里的另一个容器服务名

---

## 整体数据流

```
mlc-go:9091/metrics ──┐
                       ├──► Prometheus (每15s/60s抓取) ──► 评估告警规则 ──► Alertmanager:9093 ──► 通知
kafka offset exp:9308 ─┘            │
                                     └──► 存储时序数据 (prometheus_data 卷)
                                                            │
                                                       Grafana 查询（如果配了）
```

简单总结：**这份配置让 Prometheus 每 15 秒抓一次你的 Go 应用指标、每 60 秒抓一次 Kafka 消费延迟，加载本地告警规则，触发的告警转发给 Alertmanager 去做通知。**


<br/>

***
<br/><br/><br/>
># <h1 id="Grafana的数据源预配置文件">Grafana的数据源预配置文件</h1>

## monitoring/grafana/provisioning/datasources/prometheus.yml 这个文件的作用

这是 Grafana 的**数据源预配置文件（datasource provisioning）**，通常放在 `grafana/provisioning/datasources/` 目录下，Grafana 启动时自动加载，**不需要手动在界面里添加数据源**。

逐段拆解：

---

### 1. 头部元信息

```yaml
apiVersion: 1
```

Grafana provisioning 配置的版本号，目前固定为 1。

---

### 2. 定义 Prometheus 数据源

```yaml
datasources:
  - name: Prometheus          # 数据源在 Grafana 界面里显示的名字
    uid: mlc-prometheus       # 固定唯一标识，仪表盘和告警规则通过这个 UID 引用数据源
    type: prometheus          # 数据源类型
    access: proxy              # 代理模式：Grafana 后端去请求 Prometheus，浏览器不直连
    url: http://prometheus:9090  # Prometheus 地址，用 Compose 服务名
    isDefault: true            # 设为默认数据源，新建面板默认选它
    editable: false            # 界面中不可修改，防止误操作
```

**关键设计点：**

| 配置 | 为什么这么设 |
|---|---|
| `uid: mlc-prometheus` | **固定 UID 是核心**。Grafana 仪表盘 JSON 和告警规则里引用数据源时用的是 UID 而不是名字。如果 UID 每次启动都变，导入的仪表盘和告警规则就会全部失效。固定 UID 保证仪表盘/告警配置可以稳定复用 |
| `access: proxy` | Grafana 服务端代理请求 Prometheus。浏览器只跟 Grafana 通信，不需要能直接访问 Prometheus 的 9090 端口，更安全 |
| `url: http://prometheus:9090` | 用 Compose 服务名 `prometheus` 而不是 `localhost`，因为 Grafana 和 Prometheus 在同一个 docker-compose 网络里，服务名可直接 DNS 解析 |
| `editable: false` | 禁止在 Grafana 界面修改这个数据源，避免团队成员误改 URL 或类型导致仪表盘全部挂掉。要改就改这个 yaml 文件重启 |
| `isDefault: true` | 新建面板时默认选中这个数据源，少点一步 |

---

### 3. 启用告警规则管理视图

```yaml
jsonData:
  manageAlerts: true
```

**作用**：在 Grafana 的 Alerting 界面中，**展示和管理 Prometheus 里已经加载的 MLC 告警规则**（就是 prometheus.yml 里 `rule_files` 加载的那些）。

开启后，你可以在 Grafana 的 `Alerting → Alert Rules` 页面看到：
- Prometheus 中定义的告警规则列表
- 每条规则的当前状态（inactive / pending / firing）
- 规则的表达式、持续时间、标签等

注意：`manageAlerts: true` 只是让 Grafana **读取和展示** Prometheus 的告警规则，不代表可以在 Grafana 里编辑它们（Prometheus 的规则是只读挂载的，改不了）。真正的告警通知路由还是由 Alertmanager 负责。

---

## 整体关系图

```
prometheus.yml 里的 rule_files
        │
        ▼
┌─────────────────┐
│   Prometheus    │  加载告警规则、评估、触发
│  (mlc-prometheus)│
└────────┬────────┘
         │ http://prometheus:9090
         │ (Compose 服务名)
         ▼
┌─────────────────┐
│     Grafana     │  通过这个 provisioning 文件自动配置数据源
│                 │  - 查询 Prometheus 数据画仪表盘
│                 │  - manageAlerts: true → 展示 Prometheus 告警规则
└─────────────────┘
```

---

## 一句话总结

**这个文件让 Grafana 启动时自动连上 Prometheus（固定 UID、不可编辑、默认数据源），并在 Grafana 告警页面里展示 Prometheus 加载的 MLC 告警规则状态，全程不需要手动在界面配置。**


<br/>

***
<br/><br/><br/>
># <h1 id="Prometheus的告警规则文件">Prometheus 的告警规则文件</h1>
### 文件位置在：monitoring/prometheus/rules/mlc.rules.yml

## 这个文件是干嘛的

这是 **Prometheus 的告警规则文件（Alerting Rules）**，放在 `monitoring/rules/` 目录下，被 `prometheus.yml` 里的 `rule_files: /etc/prometheus/rules/*.yml` 加载。

**核心作用**：定义了 **12 条 Kafka 相关的告警规则**，Prometheus 每 15 秒（`evaluation_interval`）执行一遍这些规则里的 PromQL 表达式，满足条件就生成告警，推给 Alertmanager 去通知。

---

## 文件结构

```yaml
groups:
  - name: mlc-kafka        # 规则组名，同一组的规则共享评估周期
    rules:
      - alert: 告警名1      # 第 1 条告警规则
        expr: PromQL表达式
        for: 持续时间
        labels: {...}
        annotations: {...}
      - alert: 告警名2      # 第 2 条
        ...
```

每条规则的四个核心字段：

| 字段 | 作用 |
|---|---|
| `expr` | PromQL 表达式，Prometheus 周期性执行，结果 > 0（或满足条件）就认为告警触发 |
| `for` | 表达式持续满足多久才真正进入 firing 状态。期间状态是 pending，不会发通知。用来**过滤瞬时抖动** |
| `labels` | 告警的元数据标签，`severity`（严重级别）、`service`、`component` 等，Alertmanager 靠这些标签做路由、分组、抑制 |
| `annotations` | 告警的描述信息，`summary`（摘要）、`runbook_url`（运维手册链接），通知消息里展示给人看 |

---

## 12 条规则逐条解读

按功能分成四类：

### 第一类：目标不可达（监控本身挂了）

**1. `MLCKafkaTargetDown`**
```yaml
expr: up{job="mlc-go"} == 0
for: 2m
severity: critical
```
- `up` 是 Prometheus 内置指标，抓取成功 = 1，失败 = 0
- **含义**：mlc-go 应用的 9091 管理端口连续 2 分钟抓不到
- **为什么 critical**：目标都挂了，后面所有 Kafka 指标都看不到了，属于"监控失明"

**11. `MLCKafkaCommittedOffsetExporterDown`**
```yaml
expr: up{job="kafka-committed-offset-exporter"} == 0
for: 5m
severity: critical
```
- **含义**：Kafka committed offset exporter 连续 5 分钟不可达
- **为什么 critical**：broker 端的已提交位点积压无法被独立观测，应用进程内指标和 broker 端指标就没法交叉验证了

---

### 第二类：各类失败计数（有失败就告警）

这几条都是同一个模式：`increase(xxx_failures_total[5m]) > 0`，即**最近 5 分钟内失败次数有增长就触发**。

**2. `MLCKafkaProduceFailures`** — 生产消息失败（warning，持续 2m）
- 应用往 Kafka 发消息失败了

**3. `MLCKafkaCommitFailures`** — 消费位点提交失败（warning，持续 2m）
- 消费完消息后提交 offset 失败，可能导致**重复消费**或**消费进度回退**

**4. `MLCKafkaHandlerFailures`** — 业务处理失败（warning，持续 2m）
- 消息已经到消费者了，但业务逻辑处理没成功（比如调用下游服务失败、数据库写入失败）

**5. `MLCKafkaDLQWriteFailures`** — 死信队列写入失败（**critical**，持续 1m）
- 消息处理失败后本来应该写入 DLQ（死信队列）留待后续补偿，结果连 DLQ 都写不进去
- **为什么 critical + for 更短**：DLQ 是失败消息的最后一道补偿入口，写不进去意味着消息**永久丢失**，比普通处理失败严重得多

---

### 第三类：严重故障（立即告警，不等待）

**6. `MLCKafkaConsumerPanics`** — 消费者 panic（critical，`for: 0m`）
- 消费处理过程中发生 panic（Go 里的运行时崩溃），被 recover 捕获了
- **`for: 0m` 意味着立即触发**，不做任何持续时间等待——panic 本身就是严重异常，不需要确认"是不是持续发生"

**7. `MLCKafkaPartitionsLost`** — 分区丢失（critical，`for: 0m`）
- 消费者组发生 rebalance 时，本实例持有的分区被剥夺/丢失
- 可能导致**消费中断**或**未提交的数据被重放**，也是立即告警

---

### 第四类：消费延迟（Lag，分 warning / critical 两级）

这是最核心的一类，监控消费速度跟不跟得上生产速度。

#### 应用进程内 Lag（来自 mlc-go 自己暴露的指标）

**8. `MLCKafkaConsumerLagWarning`**
```yaml
expr: sum by (service, environment, group, topic) (mlc_kafka_consumer_lag_records) > 1000
for: 10m
severity: warning
```
- 按服务、环境、消费组、topic 聚合，积压超过 **1000 条**且持续 **10 分钟** → warning

**9. `MLCKafkaConsumerLagCritical`**
```yaml
expr: sum by (...) (mlc_kafka_consumer_lag_records) > 10000
for: 5m
severity: critical
```
- 积压超过 **10000 条**且持续 **5 分钟** → critical
- **设计意图**：critical 触发后，Alertmanager 会**抑制（inhibit）同维度的 warning**，避免同一问题同时发两条通知。注释里也写了："critical 触发后由 Alertmanager 抑制同维度 warning"

#### Broker 端 Committed Lag（来自 kafka-committed-offset-exporter）

**12. `MLCKafkaCommittedLagWarning`**
```yaml
expr: sum by (service, environment, consumergroup, topic) (kafka_consumergroup_lag) > 1000
for: 15m
severity: warning
```

**13. `MLCKafkaCommittedLagCritical`**
```yaml
expr: sum by (...) (kafka_consumergroup_lag) > 10000
for: 10m
severity: critical
```

这两条和上面的 Lag 规则类似，但数据源不同——`kafka_consumergroup_lag` 是 exporter 从 **Kafka broker 端**查询的已提交位点 lag，而不是应用进程内计算的。

**为什么要两套 Lag 监控？**
- 进程内 lag：反映应用"当前正在处理的消息"和"最新消息"的差距，实时但可能不准确（比如应用刚启动还没拉完）
- broker 端 committed lag：反映 Kafka broker 记录的"消费组已提交位点"和"最新消息"的差距，是权威数据，但有 commit 延迟
- 两者交叉验证：如果进程内说没 lag 但 broker 端说有很大 lag → 可能是**消费了但没提交 offset**（commit 逻辑有 bug）

---

## 几个值得注意的设计细节

### 1. `for` 时长的梯度设计
| 严重程度 | 典型 for 时长 | 逻辑 |
|---|---|---|
| 立即告警（panic、分区丢失） | `0m` | 发生即通知，不等待 |
| critical（DLQ 失败、目标 down） | `1m` / `2m` / `5m` | 短暂容忍，但快速升级 |
| warning（普通失败、lag） | `2m` / `10m` / `15m` | 充分过滤瞬时抖动，避免告警风暴 |

### 2. `sum by (...)` 聚合 — 避免高基数标签
```yaml
sum by (service, environment, group, topic) (mlc_kafka_consumer_lag_records)
```
原始指标 `mlc_kafka_consumer_lag_records` 可能带有 `partition`（分区号）标签，如果直接按分区告警，一个有 100 个分区的 topic 会产生 100 条独立告警。

用 `sum by (service, environment, group, topic)` 把分区维度聚合掉，**只按消费组 + topic 级别告警**，减少告警数量。注释里也明确写了："避免 partition 等高基数标签进入告警"。

### 3. severity 两级 + Alertmanager 抑制
- warning 和 critical 两套阈值（1000 / 10000）
- critical 触发时，Alertmanager 通过 inhibit_rule 自动抑制同维度的 warning
- 结果：严重时只收到一条 critical 通知，不会同时收到 warning + critical 两条

### 4. `runbook_url` 只在第一条规则里有
`MLCKafkaTargetDown` 配了 `runbook_url: https://runbooks.internal/mlc/target-down`，其他规则没配。这是因为目标 down 是最基础的故障，排查路径标准化程度高；其他业务类告警的 runbook 可能还没写或者在别的地方维护。

---

## 整体告警流转

```
prometheus.yml 加载本文件
        │
        ▼
Prometheus 每 15s 评估 12 条规则
        │
   满足 expr 条件？
        │
   ┌────┴────┐
   │ 否       │ 是
   │ 继续     │ 进入 pending
   └────┬────┘
        │
   持续 for 时长？
        │
   ┌────┴────┐
   │ 否       │ 是
   │ 回退     │ 进入 firing → 推给 Alertmanager
   └─────────┘        │
                      ▼
              Alertmanager 根据 severity 路由
                      │
            ┌─────────┴─────────┐
            │ warning            │ critical
            │ 低优先级通知       │ Webhook / 即时通知
            └────────────────────┘
```

---

## 一句话总结

**这个文件定义了 mlc-go 服务围绕 Kafka 的 12 条告警规则，覆盖"监控目标挂了、生产/消费/提交/DLQ 失败、消费者 panic 和分区丢失、消费延迟（进程内 + broker 端双源验证）"四大场景，通过 warning/critical 两级阈值和 for 持续时间过滤抖动，最终由 Prometheus 评估、Alertmanager 路由通知。**
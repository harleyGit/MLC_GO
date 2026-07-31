# MLC 监控配置

本目录用于配置本地 Docker Compose 监控栈，包括 Prometheus 指标抓取与告警规则、Alertmanager 通知路由，以及 Grafana Prometheus 数据源自动配置。

## 文件说明

- `prometheus.yml`：配置指标抓取、规则加载和 Alertmanager 地址。
- `alertmanager.yml`：配置告警聚合、抑制、分级路由和 Webhook 通知。
- `rules/mlc-kafka.rules.yml`：定义 Kafka 生产、消费、提交、死信队列、committed-offset lag 和目标可用性告警。
- `rules/mlc-statistic.rules.yml`：定义 ClickHouse 权威写入、Redis 投影和数据对账告警。
- `grafana/provisioning/datasources/prometheus.yml`：在 Grafana 启动时自动创建 Prometheus 数据源。
- `secrets/alertmanager_webhook_url.example`：Webhook URL 示例；复制为实际 Secret 文件后替换为真实通知地址。
- `secrets/alertmanager_webhook_url`：Docker Compose 默认读取的本地 Secret 文件，已被 `.gitignore` 排除。

## Secret 文件约束

`alertmanager_webhook_url` 和它的示例文件必须只包含一行完整 URL，不能在文件顶部或行尾添加注释。Alertmanager 的 `url_file` 会将文件内容整体作为 URL 读取，加入注释会导致通知地址解析失败。

生产环境应通过部署平台或 Secret 管理系统提供该文件，不要将真实 Webhook URL、Token 或其他凭据提交到仓库。

## Kafka 消费积压

`mlc_kafka_consumer_lag_records` 是应用观测的处理积压。它由每个消费运行器显式注册，内部按分区维护高水位与处理进度，对外仅暴露 `group`、`topic` 标签。成功处理和成功写入 DLQ 的终止错误会推进进度；可重试错误不会推进，运行器退出时对应指标会被清理。

告警在 Prometheus 中按 `service`、`environment`、`group`、`topic` 汇总所有实例：积压持续 10 分钟超过 1000 条触发 warning，持续 5 分钟超过 10000 条触发 critical。同一维度 critical 生效时，Alertmanager 会抑制 warning。

`kafka_consumergroup_lag` 是 `kafka-committed-offset-exporter` 独立读取 Kafka broker 高水位和 consumer group 已提交 offset 后计算的积压，不依赖 mlc-go 进程内状态。Prometheus 每 60 秒抓取一次，并按 `service`、`environment`、`consumergroup`、`topic` 汇总分区，避免 `partition` 标签进入告警：积压持续 15 分钟超过 1000 条触发 warning，持续 10 分钟超过 10000 条触发 critical。

Compose 配置仅用于本地开发和验收。生产部署需要使用受限监控网络、服务发现、Kafka 鉴权与 TLS，并根据消费速率、恢复时间目标和 exporter 对集群的实际开销调整抓取周期及阈值；不能将本地端口映射视为生产可用性方案。

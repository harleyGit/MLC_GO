# MLC 监控配置

本目录用于配置本地 Docker Compose 监控栈，包括 Prometheus 指标抓取与告警规则、Alertmanager 通知路由，以及 Grafana Prometheus 数据源自动配置。

## 文件说明

- `prometheus.yml`：配置指标抓取、规则加载和 Alertmanager 地址。
- `alertmanager.yml`：配置告警聚合、抑制、分级路由和 Webhook 通知。
- `rules/mlc-kafka.rules.yml`：定义 Kafka 生产、消费、提交、死信队列和目标可用性告警。
- `rules/mlc-statistic.rules.yml`：定义 ClickHouse 权威写入、Redis 投影和数据对账告警。
- `grafana/provisioning/datasources/prometheus.yml`：在 Grafana 启动时自动创建 Prometheus 数据源。
- `secrets/alertmanager_webhook_url.example`：Webhook URL 示例；复制为实际 Secret 文件后替换为真实通知地址。
- `secrets/alertmanager_webhook_url`：Docker Compose 默认读取的本地 Secret 文件，已被 `.gitignore` 排除。

## Secret 文件约束

`alertmanager_webhook_url` 和它的示例文件必须只包含一行完整 URL，不能在文件顶部或行尾添加注释。Alertmanager 的 `url_file` 会将文件内容整体作为 URL 读取，加入注释会导致通知地址解析失败。

生产环境应通过部署平台或 Secret 管理系统提供该文件，不要将真实 Webhook URL、Token 或其他凭据提交到仓库。

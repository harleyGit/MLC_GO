# MLC_GO 工程规范

## 项目与规则边界

MLC_GO 是 MLC 后端，提供业务/运维 API、事件消费和数据投影。本文件记录始终适用的项目事实与硬约束；通用方法按任务读取 `~/HGFiles/GitHub/AITools/Skills/mlc-engineering/go/SKILL.md` 及其 common 引用，不强制加载全部技能。中文交付，标识符与路径保留原样；交付与提交遵循 `~/HGFiles/GitHub/AITools/Skills/dev_general_skill/SKILL.md`。

## 版本与关键依赖

- module `MLC_GO`，`go.mod` 声明 Go `1.24.1`，Docker 构建镜像 `golang:1.26.4`；两者不擅自统一，也不据此推断部署版本。
- 关键依赖：`net/http`、项目 router/middleware、MySQL、Redis、Kafka、ClickHouse；版本以实际 manifest 和配置为准。

## 目录与架构

- `main_production.go` 使用 `production` tag，调用 `mlc_main()`；`main.go` 使用 `!production` 并依赖学习代码。
- `main_mlc_project.go` 装配基础设施和路由，管理业务 HTTP、管理探活/指标、弹幕实时服务及后台任务的生命周期。
- `internal/modules/` 按 module/handler/service/repository/cache/dto/model/mapper/task 分工；与 `internal/handler/`、`internal/repository/`、`internal/models/` 并存，不强行统一。
- `internal/events/` 定义事件；`internal/outbox/` 处理事务事件；`internal/consumer/` 实现投影；`internal/infrastructure/` 适配基础设施；`internal/pkg/` 提供配置、日志、路由、MySQL、Redis、Kafka、ClickHouse 等公共能力。
- `cmd/hg_config_check/` 是配置/依赖检查及迁移辅助命令；`cmd/hg_crawler/` 是爬虫命令。`cmd/server/main.go` 目前只有 `package server`，不是可运行主服务入口。
- `config/base/`、`config/debug/`、`config/pre/`、`config/prod/` 保存分环境配置；`migrations/` 保存数据库迁移；`deployments/` 保存部署、ClickHouse DDL、事件契约及监控；`scripts/` 保存运维脚本；`docs/` 保存 Swagger 产物。
- `TestNotes/**` 是学习/示例目录，默认不读取、不修改、不修复、不编译、不测试、不 lint、不 review。除非明确授权，不为其失败调整业务代码或规则。

## 命名与修改边界

- 新文件、类型、函数、变量、常量默认以 `HG` 或 `hg` 开头；包名遵循 Go 小写短名。接口、第三方/标准库签名及已有公共命名优先兼容，不强加前缀。
- 复用已有 API 路径、业务术语、字段/tag、缓存 key、错误码、注释和日志风格；结构体涉及协议时说明协议名称、用途、约束，导出入口说明职责与用法。
- 不擅改公共接口、协议字段、错误语义、业务顺序或缓存序列化；模块边界、并发模型、事务/幂等/重试/回滚、依赖及安全组件变更，须说明影响并获确认。
- 不得读取无关凭据或输出/提交密码、Token、密钥、完整连接串和个人隐私；连续两次编译、测试或修复失败时停止自循环，保留现场并报告阻塞。

## 数据与性能硬约束

- 以千万级高并发、亿级以上表为风险评估基线，先保证正确，不视为已验证容量；生产级要求须评估容量边界、热点、限流、降级及失败模式，性能结论附实测证据。
- 固定 Go SQL 集中在 `internal/pkg/mysql/queries/hg_sql_queries.go` 或同目录领域文件，如 `hg_video_danmaku_queries.go`；repository/DAO 只引用常量，动态条件使用集中片段。临时用途也不豁免。
- 大表 SQL 常量旁说明用途及索引、分页、事务、批量或降级前提；联查过滤/排序/Join 索引、类型、外键、扫描顺序和连接池压力，未知生产索引须报告。
- 列表有稳定排序、limit 和最大页大小，优先游标；写入限制范围，批处理分批且可恢复。全表扫描、无索引过滤、深 offset、N+1、无限 preload、长事务及热点行更新按高风险处理；禁止无保护大范围写及事务内外部 I/O。
- 数据库、业务和 API 模型不混用，不随意合并/拆分。
- Redis key 前缀集中在 `internal/pkg/redis/hg_redis_key.go`，业务只组合具体 ID；Lua 集中在 `hg_redis_script.go` 或同目录领域文件并说明 KEYS/ARGV。
- Redis 字符串可能经过 JSON 序列化时按既有协议解码再比较；写后按既有策略失效单体、列表分页及 total 缓存，保留延迟双删/消息失效语义，明确一致性窗口、TTL、原子性、热点、击穿/穿透/雪崩及失败清理策略。
- DB、Redis、队列、外部 I/O 和 API 热点评估延迟、锁竞争、超时取消、限流、幂等及降级；goroutine、队列、缓存和请求体有上限及满载策略。昂贵循环 I/O 优先批量或限并发，统计/计数/排行先评估缓存、预聚合和异步汇总。

## Statistic 不变量

- 当前处理视频 reviewed、published、deleted 事件，校验 EventID 和 Kafka delivery 坐标。配置权威存储时先写 ClickHouse，再推进 Redis；失败向上传递，成功前不得提交 Kafka offset。
- 默认 generation `v2`、64 分片，以实际配置为准；generation、shard 映射及 counter/watermark 的 Cluster hash tag 必须一致。
- Redis 实际按 `topic:partition + offset` 水位去重，依赖分区顺序，不是 EventID 去重；ClickHouse 聚合按维度使用 `uniqExactState(event_id)`。同 EventID 不同 offset 可能出现计数漂移，不混淆口径。
- 对账缺失值按零处理，只检测和报告漂移，不在线覆盖 Redis。验收会写真实 Kafka、ClickHouse、Redis，不是只读检查。

## Danmaku 不变量

- MySQL 是热数据权威；启用 Outbox topic 时弹幕、64 分片计数和 Outbox 同一短事务提交，topic 为空保留兼容路径。
- 按用户和 requestID 幂等，重放校验视频、位置、内容和样式，否则冲突；不重复计数/广播。只广播已提交的新记录，广播失败不推翻权威写入。
- 专用 topic `mlc.video.danmaku.created.v1` 保持同视频分区顺序；分区批次先批量写 ClickHouse，再推进 Redis recent，两者成功才允许提交 offset。Pub/Sub 仅为实时副本，recent 是有界近期数据，均不能代替历史。
- HTTP MySQL 游标 `(progress_ms,id)` 与 ClickHouse `(progress_ms,created_at,danmaku_id)` 不可互换。时间窗默认 60 秒、最大 5 分钟；页默认 200、最大 1000；总数最多聚合 64 个计数分片。
- 内容 1-100 字符，requestID 1-64 字符，字符不是字节。WebSocket 票据使用 32 字节随机数，绑定用户/视频，Lua 原子取出并删除，禁止重放。
- 各 Pod active rooms 之和不是全局去重房间数；queued 不是已发送/已展示。指标标签禁止 video/user/request/connection ID 及错误文本；区分 WebSocket 连接与 TCP admission。

## 验证与启动

- 根入口构建/测试必须 `-tags production`；仅过滤 `go test ./...` 包列表不能消除默认根入口的学习依赖。不要直接运行未隔离 `TestNotes` 的 `make test`、`make lint` 或全目录格式化 `make tool`。
- 按任务选择以下入口，产物留系统临时目录或工程外，不扩大 `.gitignore`：

```bash
go build -tags production -o /tmp/mlc-go .
go test -tags production .
go test ./internal/consumer/statistic ./internal/consumer/danmaku ./internal/modules/video_danmaku/... -count=1
```

- 主服务从项目根运行：`SERVER_ENV=debug go run -tags production .`；`production` tag 选择入口，不等于 `SERVER_ENV=prod`。启动会连接基础设施并运行后台任务，须先确认环境。
- 配置初始化在 `internal/pkg/config/env.go`：进程环境变量优先于可选 `config/MLC.local.env`，再读取共享 `config/MLC.env`；业务配置按 base 与环境加载，不复制真实凭据到文档。
- `./scripts/ensure_debug_deps.sh debug` 会检查/启动本地依赖、运行 debug 迁移、检查三节点 Kafka 并执行 `make kafka-init`，需要明确授权，不作为只读验证。VS Code debug 前置任务调用此脚本。
- 脚本 `pre` 模式可启动本地 MySQL/Redis compose；`prod` 只检查。pre/prod 不自动迁移或管理 Kafka Topic，Topic 由发布系统/IaC 管理。不能笼统称 pre 全部只读。
- `make statistic-acceptance` 启用 `MLC_STATISTIC_INTEGRATION=1` 并写真实外部服务；仅获准隔离环境运行。不得擅自执行迁移、初始化、重置、生产清理或集成验收。

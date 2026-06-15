# AGENTS.md

本文件是规则入口索引。执行细则由 `.agent_runtime.json` 负责装配，由 `.ai_agents/*.md` 分模块维护。

## 加载规则
- 基础任务：加载 `00-core`、`01-style`、`05-validation`、`06-output`、`07-forbidden`、`08-performance`
- Go 代码任务：在基础任务上追加 `02-go-rules`
- API / Handler / Service / DTO 任务：在 Go 代码任务上追加 `03-api-rules`
- Repository / DAO / DB / SQL / ORM / Cache 任务：在 Go 代码任务上追加 `04-data-rules`
- 同时涉及 API 与数据层时，同时加载 `03-api-rules` 与 `04-data-rules`

## 模块职责
- `00-core`：项目目标、工作边界、默认执行方式
- `01-style`：命名、注释、日志和风格一致性
- `02-go-rules`：Go 语言实现、错误、context、并发和 tag 规则
- `03-api-rules`：Handler、Service、Request、Response、校验与错误规则
- `04-data-rules`：数据库、SQL、ORM、缓存、数据模型和兼容规则
- `05-validation`：格式化、编译、测试、自检和临时产物规则
- `06-output`：交付说明与验证结果表达规则
- `07-forbidden`：禁止事项与高风险操作确认边界
- `08-performance`：容量、并发、性能、热点和生产级约束

## 冲突优先级
1. 用户明确要求
2. 安全、数据正确性、兼容性、生产稳定性要求
3. 涉及数据库增删改查、缓存一致性、亿级数据表或高并发热点路径时，`04-data-rules` 与 `08-performance` 优先
4. `.agent_runtime.json` 的装配配置
5. `.ai_agents/*.md` 的具体模块规则
6. 本入口索引
7. 项目现有实现和局部风格

冲突时选择主流、稳定、低风险、可验证的方案；无法低风险判断时先说明取舍并等待确认。

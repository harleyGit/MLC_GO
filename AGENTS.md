# AGENTS.md

入口索引。具体执行规则见 `.agent_runtime.json` 与 `.ai_agents/*.md`，本文件只保留加载关系和冲突优先级。

## 加载
- 基础任务：加载 `00-core`、`01-style`、`05-validation`、`06-output`、`07-forbidden`、`08-performance`
- Go 代码任务：追加 `02-go-rules`
- API / Handler / Service / DTO 任务：追加 `03-api-rules`
- Repository / DAO / DB / SQL / ORM 任务：追加 `04-data-rules`

## 模块职责
- `00-core`：核心目标、项目边界、默认工作方式
- `01-style`：代码风格与一致性
- `02-go-rules`：Go 语言实现规则
- `03-api-rules`：接口、处理器、服务和 DTO 规则
- `04-data-rules`：数据库、SQL、ORM、缓存与数据模型规则
- `05-validation`：格式化、编译、测试和验证规则
- `06-output`：交付说明格式
- `07-forbidden`：禁止事项与高风险操作边界
- `08-performance`：容量、并发、性能和生产级约束

## 优先级
1. 用户要求
2. 涉及数据库增删改查、亿级数据表、千万级高并发或生产级要求时，`08-performance` 与 `04-data-rules` 为最高工程约束
3. `.agent_runtime.json` 模型配置
4. `.ai_agents/*.md` 模块规则
5. 本入口索引
6. 项目现有实现

冲突时优先主流、稳定、低风险方案。

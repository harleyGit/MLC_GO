# AGENTS.md

> 入口文件，通用规则。模型差异化配置见 `.agent_runtime.json`。

## 配置

| 文件 | 作用 |
|------|------|
| `.agent_runtime.json` | 模型配置、加载规则、行为差异 |
| `.ai_agents/*.md` | 共享规则模块 |

## 默认加载

所有任务默认加载：
- `AGENTS.md`
- `.ai_agents/00-core.md`
- `.ai_agents/01-style.md`
- `.ai_agents/05-validation.md`
- `.ai_agents/06-output.md`
- `.ai_agents/07-forbidden.md`
- `.ai_agents/08-performance.md`

按类型追加：
- Go：`.ai_agents/02-go-rules.md`
- API / Handler / Service：`.ai_agents/03-api-rules.md`
- Repository / DAO / DB / SQL / ORM：`.ai_agents/04-data-rules.md`
- Shell / Bash：补足流程、分支、失败处理注释

## 优先级

1. 用户要求
2. `.agent_runtime.json` 中模型特定配置
3. 本文件
4. `.ai_agents/00-core.md`
5. 其他 `.ai_agents/*.md`
6. 项目现有实现

冲突时优先：更保守、更兼容、更主流、更稳定的方案。

## 工作流

1. 读任务和规则
2. 修改前用 `grep` / `glob` 搜索相关上下文、调用链和相似实现
3. 优先复用现有实现，减少无关重构
4. SQL 变更先验证语法、目标库、影响范围，确认后再执行
5. 格式化、自检、尽量编译/测试
6. 连续 3 次编译、测试或修复失败时停止自我循环，说明困境并向用户求助
7. 按 `.ai_agents/06-output.md` 输出，未验证的结果不得说已通过

## 代码原则

- 按高并发后端工程处理
- 命名延续 Go API 风格
- 高并发链路必须正确传递 `context.Context`，支持超时、取消和链路追踪
- 错误处理必须保留上下文，优先使用 `fmt.Errorf("doing sth: %w", err)` 包装下层错误
- 新增或修改方法、函数、关键变量时补简洁注释
- 优先主流稳定架构和常用优秀设计模式
- 发现不适合高并发的实现：先短建议，获同意后再改
- Redis 字符串值若可能经 JSON 序列化后入库，读取后比较前先做解码兼容
- 涉及会影响查询结果的数据写操作后，按现有 key 规则删除单体缓存、列表分页缓存和 total 缓存
- 高并发链路优先消除重复计算，保证语义不变
- 不做无关重构
- 未验证的结果不得说已通过

## SQL 规则

- 固定 SQL 语句优先统一写入 `internal/infrastructure/persistence/mysql/queries/hg_sql_queries.go`；文件过大时允许按业务领域平行拆分，如 `user_queries.go`
- Repository / DAO / Service / Handler 不直接散写固定 SQL 字面量
- 只有动态拼接的查询条件或极少量临时语句才允许留在调用处，并要尽量收敛
- SQL 相关改动必须同步检查：字段类型、索引、外键、查询结果扫描顺序

## 风险控制

默认不做：
- 大重构
- 改目录或模块边界
- 改公共接口、字段名、tag
- 引第三方依赖
- 改并发模型
- 改事务、幂等、重试、回滚
- 改响应结构或错误语义
- 在日志中打印密码、Token、密钥、用户隐私数据等敏感信息

## 临时文件

- 不把缓存、日志、二进制写入工程目录
- 优先用系统临时目录或工程外目录

## 收尾

- 自检
- 格式化
- 尽量编译/测试
- 真实说明改动与验证

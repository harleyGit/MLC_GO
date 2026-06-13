# AGENTS.md

入口索引，具体规则见 `.agent_runtime.json` 与 `.ai_agents/*.md`。

## 加载
- 基础：`00-core`、`01-style`、`05-validation`、`06-output`、`07-forbidden`、`08-performance`
- Go：追加 `02-go-rules`
- API / Handler / Service / DTO：追加 `03-api-rules`
- Repository / DAO / DB / SQL / ORM：追加 `04-data-rules`

## 范围
- `/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO/TestNotes/**` 为学习/示例文件，不纳入日常修改、编译、测试、lint、review 范围
- 除非用户明确要求处理 `TestNotes`，否则不要读取、修改、修复或为其测试失败做适配
- 执行 `go test` / `go test ./...` 前优先选择受影响业务包；若全量命令包含 `TestNotes` 并失败，应标注为已知排除范围，不作为本次业务验证失败

## 数据与并发基线

### 容量与并发默认值
- 所有数据库增删改查默认按数据库表中数据亿级以上规模设计，不按小表或后台低频场景假设
- 涉及数据库、缓存、队列、外部 I/O、API 热点路径时，默认按千万级并发生产约束评估容量、延迟、锁竞争、热点、限流、降级和失败模式

### CRUD 设计检查
- CRUD 改动必须优先检查索引命中、分页方式、事务范围、批量策略、连接池压力、缓存一致性和并发安全
- 针对单表数据量千万级及以上的增删改查，必须把索引命中、查询条件、分页策略、事务边界、批量写入/删除策略和降级限制写入对应业务模块文件的代码注释或实现说明中，避免只在外部文档描述。

### SQL 放置与注释
- Go 业务代码中的固定 SQL 必须集中放入 `internal/pkg/mysql/queries/hg_sql_queries.go`，Repository / DAO 中只引用 SQL 常量；只有动态拼接的条件片段可以通过该文件中的 SQL 片段常量组合。
- 新增或修改 SQL 常量时，必须在 SQL 常量旁写明用途、索引命中、分页策略、事务边界或降级限制；涉及千万级表时不得只在调用处解释。
- 禁止在 Repository / DAO 中散落大段内联 SQL；如确需临时 SQL，必须写明原因和后续迁移到 `hg_sql_queries.go` 的条件。

## 优先级
1. 用户要求
2. 涉及数据库增删改查、数据库表中数据亿万、千万级高并发/高并发/生产级要求时，`08-performance` 与 `04-data-rules` 为最高工程约束
3. `.agent_runtime.json` 模型配置
4. 本文件与 `.ai_agents/*.md`
5. 项目现有实现

冲突时优先主流、稳定、低风险方案。

## 禁止事项
- **禁止将编译产物、中间文件放到工程目录中**（如 `mlc_server`、`*.exe`、`*.o`、`*.a` 等二进制文件）
- 编译时使用 `-o /tmp/` 或其他非工程目录输出编译产物
- 确保 `.gitignore` 包含所有可能的编译产物

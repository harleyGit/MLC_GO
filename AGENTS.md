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
- 所有数据库增删改查默认按数据库表中数据亿级以上规模设计，不按小表或后台低频场景假设
- 涉及数据库、缓存、队列、外部 I/O、API 热点路径时，默认按千万级并发生产约束评估容量、延迟、锁竞争、热点、限流、降级和失败模式
- CRUD 改动必须优先检查索引命中、分页方式、事务范围、批量策略、连接池压力、缓存一致性和并发安全

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

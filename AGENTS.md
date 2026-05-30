# AGENTS.md

入口索引，具体规则见 `.agent_runtime.json` 与 `.ai_agents/*.md`。

## 加载
- 基础：`00-core`、`01-style`、`05-validation`、`06-output`、`07-forbidden`、`08-performance`
- Go：追加 `02-go-rules`
- API / Handler / Service / DTO：追加 `03-api-rules`
- Repository / DAO / DB / SQL / ORM：追加 `04-data-rules`

## 优先级
1. 用户要求
2. 明确千万级高并发/高并发/生产级要求时，`08-performance` 为最高工程约束
3. `.agent_runtime.json` 模型配置
4. 本文件与 `.ai_agents/*.md`
5. 项目现有实现

冲突时优先主流、稳定、低风险方案。

## 禁止事项
- **禁止将编译产物、中间文件放到工程目录中**（如 `mlc_server`、`*.exe`、`*.o`、`*.a` 等二进制文件）
- 编译时使用 `-o /tmp/` 或其他非工程目录输出编译产物
- 确保 `.gitignore` 包含所有可能的编译产物

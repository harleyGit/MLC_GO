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
- API / Handler / Service / DTO / Request / Response：`.ai_agents/03-api-rules.md`
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

1. 先读任务、规则、上下文、调用链和相似实现
2. 优先复用现有实现，保持最小必要改动
3. SQL / 数据 / 高并发 / 安全相关改动按对应模块规则执行
4. 完成后格式化、自检、尽量编译/测试，并真实说明验证结果

## 代码原则

- 按高并发后端工程处理
- 命名延续 Go API 风格
- 优先主流稳定架构和常用优秀设计模式
- 发现不适合高并发的实现：先短建议，获同意后再改
- 不做无关重构
- 未验证的结果不得说已通过

## 风险控制

默认不做：
- 大重构
- 改目录或模块边界
- 改公共接口、字段名、tag
- 引第三方依赖
- 改并发模型
- 改事务、幂等、重试、回滚
- 改响应结构或错误语义
- 输出密码、Token、密钥、用户隐私数据等敏感信息

## 临时文件

- 不把缓存、日志、二进制写入工程目录
- 优先用系统临时目录或工程外目录

## 收尾

- 自检
- 格式化
- 尽量编译/测试
- 真实说明改动与验证

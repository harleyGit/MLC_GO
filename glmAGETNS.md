

# AGENTS.md

## AI 工具入口
本文件为 AI 编程助手规则入口，支持多种工具。

## 目录结构
```
.glm/           # GLM (opencode) 规则
├── rules.md    # 核心规则入口
├── go.md       # Go 规则
├── api.md      # API 规则
├── data.md     # 数据层规则
├── validation.md # 验证规则
├── output.md   # 输出格式规则
├── forbidden.md # 禁止项
└── performance.md # 高并发性能规则

.ai_agents/     # Codex 规则（精简版）
├── 00-core.md      # 核心
├── 01-style.md     # 风格
├── 02-go-rules.md     # Go
├── 03-api-rules.md`  # API
├── 04-data-rules.md` # 数据层
├── 05-validation.md` # 验证
├── 06-output.md`     # 输出
├── 07-forbidden.md`  # 禁止项
└── 08-performance.md` # 高并发性能
```

## GLM 使用方式
GLM 自动加载本文件，核心规则位于 `.glm/rules.md`。

按任务类型 GLM 应主动读取：
- Go 代码 → `.glm/go.md`
- API / Handler / Service → `.glm/api.md`
- Repository / DAO / DB → `.glm/data.md`
- 需要验证 → `.glm/validation.md`
- 需要输出格式 → `.glm/output.md`
- 涉及禁止项 → `.glm/forbidden.md`
- 性能优化 → `.glm/performance.md`

## Codex 使用方式
Codex 自动加载本文件，按任务类型读取 `.ai_agents/` 目录：
- Go 代码 → `01-go.md`
- API / Handler / Service → `02-api.md`
- Repository / DAO / DB → `03-data.md`
- 性能优化 → `04-performance.md`
- 需要验证 → `05-validation.md`

## 优先级
1. 用户要求
2. 本文件 (AGENTS.md)
3. 对应工具规则目录 (.glm/ 或 .ai_agents/)
4. 项目现有实现

冲突时选：
- 更保守
- 更兼容
- 更优和主流设计

## 默认加载
所有任务：
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

## 默认流程
1. 读任务
2. 读规则
3. 看上下文和相似实现
4. 优先做最优和主流设计改动
5. SQL 变更先做验证（语法、目标库、影响范围），确认无误后再执行
6. 格式化、自检、尽量编译/测试
7. 按 `.ai_agents/06-output.md` 输出

## 默认要求
- 按高并发后端工程处理
- 先复用现有实现
- 优先主流稳定架构和常用优秀设计模式
- 发现不适合高并发的实现：先短建议，获同意后再改
- 命名延续 Go API 风格
- 新增或修改方法、函数、关键变量：补简洁注释
- Redis 字符串值若可能经 JSON 序列化后入库（如验证码），读取后比较前先做解码兼容，不直接拿原值比较
- 涉及会影响列表结果的数据写操作（如用户注册）后，按现有 key 规则删除对应列表分页缓存和 total 缓存
- 高并发链路优先消除重复计算（比如：重复鉴权、重复 panic 恢复、重复 body 读取），保证语义不变
- 不做无关重构
- 未验证的结果不得说已通过

## 风险
默认不做：
- 大重构
- 改目录或模块边界
- 改公共接口、字段名、tag
- 引第三方依赖
- 改并发模型
- 改事务、幂等、重试、回滚
- 改响应结构或错误语义

## 临时文件
- 不把缓存、日志、二进制写入工程目录
- 优先用系统临时目录或工程外目录

## 收尾
- 自检
- 格式化
- 尽量编译/测试
- 真实说明改动与验证

****
<br/><br/><br/><br/>

有`‌.agent_runtime.json`文件的加载，后面使用处理文本的高级模型进行处理。GLM不行，详细看：https://chatgpt.com/s/t_69f1a7e348b88191bd7685112d2285f7


# AGENTS.md

## common（所有模式共享）
本区块为统一规则，由 runtime 决定加载策略。

### 优先级
1. 用户要求
2. 本文件
3. 子规则目录
4. 项目现有实现

冲突策略：
- 更保守
- 更兼容
- 更小改动

---

### 默认流程（统一）
1. 读任务
2. 读规则
3. 看上下文
4. 最小必要改动
5. 自检 + 格式化
6. 按 output 规则输出

---

### 默认要求（统一）
- 高并发后端标准
- 优先复用
- 禁止无关重构
- 不得伪造验证结果

---

### 高并发约束（统一）
- 消除重复计算
- 避免重复 IO
- 避免重复鉴权
- Redis 读取需兼容 JSON

---

## codex（仅 Codex 生效）
由 runtime.mode=codex 激活

加载来源：
.ai_agents/

规则映射：
- Go → 02-go-rules.md
- API → 03-api-rules.md
- Data → 04-data-rules.md

特点：
- 偏工程实现
- 偏代码生成
- 偏结构约束

---

## glm（仅 GLM 生效）
由 runtime.mode=glm 激活

加载来源：
.glm/

规则映射：
- Go → go.md
- API → api.md
- Data → data.md

特点：
- 偏语义理解
- 偏流程推理
- 偏规则解释

---

## hybrid（推荐）
同时启用 codex + glm

策略：
- GLM → 理解任务
- Codex → 生成代码
- 冲突 → 取保守方案










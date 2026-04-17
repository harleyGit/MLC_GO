# AGENTS.md

## 入口
- 先读本文件，再读 `.ai_agents/` 对应模块。

## 文件
- `AGENTS.md`：总规则
- `.ai_agents/00-core.md`：核心
- `.ai_agents/01-style.md`：风格
- `.ai_agents/02-go-rules.md`：Go
- `.ai_agents/03-api-rules.md`：API
- `.ai_agents/04-data-rules.md`：数据层
- `.ai_agents/05-validation.md`：验证
- `.ai_agents/06-output.md`：输出
- `.ai_agents/07-forbidden.md`：禁止项

## 优先级
1. 用户要求
2. `AGENTS.md`
3. `.ai_agents/00-core.md`
4. 其他 `.ai_agents/*.md`
5. 项目现有实现

冲突时选：
- 更保守
- 更兼容
- 更小改动

## 默认加载
所有任务：
- `AGENTS.md`
- `.ai_agents/00-core.md`
- `.ai_agents/01-style.md`
- `.ai_agents/05-validation.md`
- `.ai_agents/06-output.md`
- `.ai_agents/07-forbidden.md`

按类型追加：
- Go：`.ai_agents/02-go-rules.md`
- API / Handler / Service：`.ai_agents/03-api-rules.md`
- Repository / DAO / DB / SQL / ORM：`.ai_agents/04-data-rules.md`
- Shell / Bash：补足流程、分支、失败处理注释

## 默认流程
1. 读任务
2. 读规则
3. 看上下文和相似实现
4. 最小必要改动
5. 格式化、自检、尽量编译/测试
6. 按 `.ai_agents/06-output.md` 输出

## 默认要求
- 按高并发后端工程处理
- 先复用现有实现
- 优先主流稳定架构和常用优秀设计模式
- 发现不适合高并发的实现：先短建议，获同意后再改
- 命名延续 Go API 风格
- 新增或修改方法、函数、关键变量：补简洁注释
- Redis 字符串值若可能经 JSON 序列化后入库（如验证码），读取后比较前先做解码兼容，不直接拿原值比较
- 涉及会影响列表结果的数据写操作（如用户注册）后，按现有 key 规则删除对应列表分页缓存和 total 缓存
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

# 02-go-rules.md

## Go
- 清理未使用 import / 变量，import 顺序按 Go 工具结果
- 若项目用 `goimports`，优先 `goimports`
- 不随意改导出 / 未导出状态
- 包命名小写短名，按职责划分，如 service、repo、api、model
- 包、结构体、导出函数必须有注释，说明职责与用法；核心函数必须有单测
- 禁止无必要全局变量，配置和依赖通过依赖注入传递

## SOLID
- 单一职责：结构体/函数/方法只负责一项核心职责，复杂逻辑拆分数据处理、业务逻辑和 I/O；函数尽量不超过 50 行
- 开闭原则：新增能力优先通过接口、组合、配置扩展，避免直接修改稳定核心函数兼容新场景
- 里氏替换：实现必须兼容接口入参、返回值、错误规则和业务约束，可无缝替换
- 接口隔离：接口小而精，建议 1~3 个强相关方法，避免胖接口和无用实现
- 依赖倒置：高层和低层都依赖抽象，使用 DI 传递依赖，便于 Mock 测试
- 迪米特法则：只与直接依赖通信，最小化暴露内部字段/方法，降低耦合
- 组合复用：优先组合/聚合而非继承式扩展，组合层级不超过 2 层

## 错误
- 必须显式返回 error，不吞错、不忽略错误
- 不改错误语义
- 下层错误向上返回时必须携带上下文，优先使用 `fmt.Errorf("doing sth: %w", err)` 包装

## context
- 沿用现有 `context.Context` 传递方式，不丢上游 context
- 请求链路不用 `context.Background()` / `context.TODO()` 替代上游 context
- 新增超时控制必须释放 cancel

## 并发
- 防泄漏、死锁、竞态、重复关 channel、无界 goroutine/channel/内存
- 限流、幂等、锁、计数等并发控制必须保证原子性
- goroutine 必须控制生命周期，使用 `context`、`sync.WaitGroup`、channel 管理取消、等待和退出
- goroutine 必须 `defer` + `recover` 防 panic 扩散；一个 goroutine 只处理一类任务
- channel 用于 goroutine 间安全通信与同步，能用方向类型时明确 `<-chan` / `chan<-`
- 发送方负责关闭 channel，接收方判断关闭状态；禁止向已关闭 channel 发送数据
- 带缓冲 channel 用于异步削峰，无缓冲 channel 用于强同步
- `select` 必须有超时、取消或退出机制，内部只做简单分发，不写复杂业务
- 优先通过 channel 通信共享内存；确需锁时缩小锁范围并说明一致性边界
- 并发调度与业务逻辑分离，生成代码必须简洁、安全、可退出、无泄漏

## tag
不得擅改 `json`、`yaml`、`form`、`mapstructure`、`gorm`、`db`、`bson`、`xml` tag。

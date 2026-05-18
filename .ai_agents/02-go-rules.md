# 02-go-rules.md

## Go
- 保持现有 package 划分和包职责
- 清理未使用 import
- import 顺序按 Go 工具结果
- 若项目用 goimports，优先 goimports

## 错误
- 沿用现有错误处理风格
- 不吞错
- 不改错误语义
- 下层错误向上返回时必须携带上下文，优先使用 `fmt.Errorf("doing sth: %w", err)` 包装

## context
- 沿用现有 context 传递方式
- 不丢上游 context
- 不随意用 `context.Background()` / `context.TODO()`
- 高并发请求链路必须传递 `context.Context`，用于超时、取消和链路追踪
- 新增超时控制时必须确保 cancel 被释放

## 并发
- 未明确要求，不主动重构并发
- 注意泄漏、死锁、竞态、重复关 channel、上下文释放、数据一致性
- 当用户要求“大厂标准”“百万级并发”“高并发”“生产级”时，不得只实现可运行逻辑；必须主动检查并修正边界突刺、goroutine 泄漏、无界 channel、无界内存、重复 body 读取、阻塞 I/O、上下文取消不生效等问题
- 高并发任务队列必须有边界：channel/队列需设容量、满队列需返回错误或降级，不允许默认无限堆积
- 限流、幂等、锁、计数等并发控制必须保证原子性；Redis 多命令组合优先使用 Lua 或事务，避免并发穿透

## 可见性
- 不随意改导出 / 未导出状态

## tag
不得擅改：
- `json`
- `yaml`
- `form`
- `mapstructure`
- `gorm`
- `db`
- `bson`
- `xml`

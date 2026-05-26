# 02-go-rules.md

## Go
- 清理未使用 import / 变量，import 顺序按 Go 工具结果
- 若项目用 `goimports`，优先 `goimports`
- 不随意改导出 / 未导出状态

## 错误
- 不吞错
- 不改错误语义
- 下层错误向上返回时必须携带上下文，优先使用 `fmt.Errorf("doing sth: %w", err)` 包装

## context
- 沿用现有 `context.Context` 传递方式，不丢上游 context
- 请求链路不用 `context.Background()` / `context.TODO()` 替代上游 context
- 新增超时控制必须释放 cancel

## 并发
- 防泄漏、死锁、竞态、重复关 channel、无界 goroutine/channel/内存
- 限流、幂等、锁、计数等并发控制必须保证原子性

## tag
不得擅改 `json`、`yaml`、`form`、`mapstructure`、`gorm`、`db`、`bson`、`xml` tag。

# 02-go-rules.md

## package 与文件组织
- 保持当前 package 划分方式一致。
- 可以移动文件、拆分目录、改变模块边界，向中高级企业开发看齐；
- 不因局部优化改变整个包职责。

## import
- 修改后必须清理未使用 import。
- import 顺序遵循 Go 标准工具结果。
- 若项目使用 goimports，优先沿用。

## 错误处理
- 错误处理方式与项目现有风格一致。
- 若已有统一错误包装、错误码、错误响应结构，必须沿用。
- 不吞掉错误。
- 不擅自改变错误返回语义。

## context
- 遵循项目现有 context 传递方式。
- 不要丢失上游 context。
- 不要随意使用 context.Background() 或 context.TODO() 覆盖真实上下文。

## 并发
涉及 goroutine、channel、mutex、rwmutex、waitgroup、atomic、timer、cancel 时必须谨慎：
- 避免 goroutine 泄漏
- 避免死锁
- 避免竞态条件
- 避免重复关闭 channel
- 避免上下文未释放
- 避免并发数据不一致

若任务未明确要求，不主动重构并发逻辑。

## 导出与可见性
- 不随意将未导出符号改为导出符号。
- 不随意将导出符号改为未导出。
- 任何可见性变化都必须谨慎处理。

## tag
以下 tag 不得擅自修改：
- json
- yaml
- form
- mapstructure
- gorm
- db
- bson
- xml

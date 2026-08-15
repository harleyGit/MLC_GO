package spider

import "time"

// HGCreateTaskRequest 创建一次受控的采集任务。
// 当前仅支持 platform=bilibili、type=recommendation；priority 范围为 1-10，非法值回落为 5。
type HGCreateTaskRequest struct {
	Platform string `json:"platform"` // 第三方平台稳定标识。
	Type     string `json:"type"`     // 任务类型，当前固定为 recommendation。
	Priority int    `json:"priority"` // 预留调度优先级；当前串行 worker 不执行抢占。
}

// HGTaskSnapshot 是面向管理端的不可变任务快照。
// 状态按 RUNNING -> SUCCESS/FAILED 单向迁移，时间统一使用 UTC，CostMillis 单位为毫秒。
type HGTaskSnapshot struct {
	ID         int64     `json:"id"`
	Type       string    `json:"type"`
	Platform   string    `json:"platform"`
	Status     string    `json:"status"`
	Priority   int       `json:"priority"`
	ItemCount  int       `json:"itemCount"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
	CostMillis int64     `json:"costMillis"`
}

package notification

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"MLC_GO/internal/pkg/logHG"
)

// HGNotificationPublisher 只发布到服务端鉴权的本人私有频道，不接受浏览器指定频道。
type HGNotificationPublisher interface {
	HGPublishUser(context.Context, string, json.RawMessage) error
}

// HGNotificationHints 共享 Kafka、MySQL 工作线程与 HTTP 投递的有损提示；不保存用户/正文。
type HGNotificationHints struct {
	mu        sync.RWMutex            // 网关在启动装配时注入，Kafka 消费线程可同时投递。
	publisher HGNotificationPublisher // nil 表示实时能力未配置，收件箱仍可用。
	lastError time.Time               // 网关故障最多每分钟记录一次错误类别。
}

// HGNewHints 构建待装配的提示器，不启动新的工作线程或网络连接。
func HGNewHints() *HGNotificationHints { return &HGNotificationHints{} }

// HGUsePublisher 在网关准备就绪时绑定已有的本人频道发布器。
func (hints *HGNotificationHints) HGUsePublisher(publisher HGNotificationPublisher) {
	if hints == nil {
		return
	}
	hints.mu.Lock()
	hints.publisher = publisher
	hints.mu.Unlock()
}

// HGNotify 仅在权威收件箱首次提交后发送无正文提示；断线/失败由本人 HTTP 补拉。
func (hints *HGNotificationHints) HGNotify(ctx context.Context, user, category string) {
	if hints == nil {
		return
	}
	hints.mu.RLock()
	publisher := hints.publisher
	hints.mu.RUnlock()
	if publisher == nil {
		return
	}
	payload, err := json.Marshal(struct {
		EventType string `json:"event_type"` // 固定的站内通知变更事件。
		Category  string `json:"category"`   // 受服务端枚举约束的分类；不含正文及用户 ID。
	}{EventType: "notification.inbox.updated.v1", Category: category})
	if err == nil {
		// 已提交的入箱不能被网关慢请求拖住任务游标或 HTTP 响应。
		publishCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		err = publisher.HGPublishUser(publishCtx, user, payload)
		cancel()
	}
	if err != nil {
		hints.mu.Lock()
		if time.Since(hints.lastError) >= time.Minute {
			logHG.ErrFInfo("站内通知已入箱，但实时提示发送失败（限速日志）：%T", err)
			hints.lastError = time.Now()
		}
		hints.mu.Unlock()
	}
}

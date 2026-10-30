package notification

import (
	"context"
	"errors"
	"strings"
)

// HGPreferences 是通知偏好协议；安全/系统通知始终可达，关闭其他分类仅影响后续发送。
type HGPreferences struct {
	Categories map[string]bool `json:"categories"` // 允许接收的互动/业务分类，system 不可关闭。
	Channels   map[string]bool `json:"channels"`   // 渠道偏好；本期仅 in_app 真正接通。
	QuietHours HGQuietHours    `json:"quietHours"` // 预留免打扰设置；本期不调度延迟发送。
	Digest     string          `json:"digest"`     // instant/hourly/daily；本期仅 instant 生效。
}

// HGQuietHours 是后续异步投递的时间窗配置；本期仅存储，不承诺定时调度。
type HGQuietHours struct {
	Enabled bool   `json:"enabled"` // true 表示用户希望开启免打扰；当前不影响站内入箱。
	Start   string `json:"start"`   // 本地 HH:mm，暂不解析为调度时区。
	End     string `json:"end"`     // 本地 HH:mm，暂不解析为调度时区。
}

var HGErrInvalid = errors.New("通知参数不合法")

var hgCategories = map[string]bool{"system": true, "comment": true, "like": true, "mention": true, "follow": true, "trade": true, "wallet": true, "marketing": true}

func hgDefaultPreferences() HGPreferences {
	return HGPreferences{
		Categories: map[string]bool{"comment": true, "like": true, "mention": true, "follow": true, "trade": true, "wallet": true, "marketing": true},
		Channels:   map[string]bool{"in_app": true, "sms": false, "email": false, "push": false, "realtime": false},
		QuietHours: HGQuietHours{Start: "22:00", End: "08:00"}, Digest: "instant",
	}
}

// HGService 仅处理站内通知；IM 消息必须走独立的会话、序列号与媒体授权链路。
type HGService struct{ repo *HGRepository }

// HGNewService 构建通知业务层，供可信事件生产者调用 HGDeliver。
func HGNewService(repo *HGRepository) *HGService { return &HGService{repo: repo} }

// HGDeliver 以稳定事件 ID 幂等生成纯文本通知；外部事件生产者须保证源事务与事件发布一致。
func (service *HGService) HGDeliver(ctx context.Context, eventID, receiver string, notice HGNotice) error {
	if len(eventID) < 1 || len(eventID) > 64 || !hgASCII(eventID) || len(receiver) < 1 || len(receiver) > 128 || !hgCategories[notice.Category] || len(notice.Title) < 1 || len(notice.Title) > 255 || len(notice.Content) < 1 || len(notice.Content) > 2048 || len(notice.TargetType) > 32 || !hgASCII(notice.TargetType) || len(notice.TargetID) > 128 || strings.ContainsAny(notice.Title+notice.Content, "<>") {
		return HGErrInvalid
	}
	if notice.Category != "system" {
		pref, err := service.repo.HGPreferences(ctx, receiver)
		if err != nil {
			return err
		}
		if !pref.Categories[notice.Category] || !pref.Channels["in_app"] {
			return nil
		}
	}
	return service.repo.HGDeliver(ctx, eventID, receiver, notice)
}

func hgASCII(value string) bool {
	for _, char := range value {
		if char < 33 || char > 126 {
			return false
		}
	}
	return true
}

// HGSavePreferences 禁止客户端关闭系统通知；未接通的外部渠道标记为关闭。
func (service *HGService) HGSavePreferences(ctx context.Context, user string, pref HGPreferences) error {
	if pref.Digest != "instant" || pref.QuietHours.Enabled || pref.Categories["system"] == false && hgHasKey(pref.Categories, "system") {
		return HGErrInvalid
	}
	for key := range pref.Categories {
		if !hgCategories[key] || key == "system" {
			return HGErrInvalid
		}
	}
	for key, value := range pref.Channels {
		if key != "in_app" && key != "sms" && key != "email" && key != "push" && key != "realtime" {
			return HGErrInvalid
		}
		if key != "in_app" && value {
			return HGErrInvalid
		}
	}
	if !pref.Channels["in_app"] {
		return HGErrInvalid
	}
	if len(pref.QuietHours.Start) > 5 || len(pref.QuietHours.End) > 5 {
		return HGErrInvalid
	}
	return service.repo.HGSavePreferences(ctx, user, pref)
}

func hgHasKey(items map[string]bool, key string) bool { _, ok := items[key]; return ok }

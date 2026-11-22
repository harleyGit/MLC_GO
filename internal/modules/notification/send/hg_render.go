// Package send 实现受限纯文本模板渲染与有界站内任务投递，不提供外部渠道或全站广播。
package send

import (
	notification "MLC_GO/internal/modules/notification"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	HGErrInvalid  = errors.New("发送参数不合法")
	HGErrConflict = errors.New("发送请求幂等冲突")
	HGErrMissing  = errors.New("发送任务或已发布模板不存在")
	HGErrBusy     = errors.New("未完成发送任务已达上限")
	hgPlaceholder = regexp.MustCompile(`\{\{([a-z][a-z0-9_]*)\}\}`)
	hgName        = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

// HGVersion 是被任务绑定的不可变模板版本；发送前重新读取已发布状态，不执行任意模板表达式。
type HGVersion struct {
	Category string          // 仅 system/marketing 与现有收件箱分类对应。
	Title    string          // 受限纯文本标题，最大 255 字节。
	Body     string          // 受限纯文本正文，最大 2048 字节。
	Schema   json.RawMessage // 变量类型与长度声明，最多 30 项。
	Action   string          // 当前只允许 none，不开放未经审核的路由。
	Config   json.RawMessage // 预留的受控跳转配置，本期必须为空对象。
	Channels json.RawMessage // 当前只允许 ["in_app"]。
}

type hgRule struct {
	Type      string `json:"type"`       // 当前只支持 string。
	Required  bool   `json:"required"`   // true 时任务必须提供值。
	MaxLength int    `json:"max_length"` // 字符上限；零时默认 256。
	Pattern   string `json:"pattern"`    // 可选完整匹配的 RE2 表达式。
}

// HGValidateVersion 只允许当前确实接通的类别、纯文本及站内渠道，不把 realtime 当作已送达。
func HGValidateVersion(v HGVersion) error {
	if (v.Category != "system" && v.Category != "marketing") || v.Action != "none" ||
		len(v.Title) == 0 || len(v.Title) > 255 || len(v.Body) == 0 || len(v.Body) > 2048 ||
		strings.ContainsAny(v.Title+v.Body, "<>") || len(v.Schema) > 8192 || len(v.Channels) > 256 || len(v.Config) > 2048 {
		return HGErrInvalid
	}
	var channels []string
	if json.Unmarshal(v.Channels, &channels) != nil || len(channels) != 1 || channels[0] != "in_app" {
		return HGErrInvalid
	}
	var config map[string]json.RawMessage
	if json.Unmarshal(v.Config, &config) != nil || len(config) != 0 {
		return HGErrInvalid
	}
	var rules map[string]hgRule
	if json.Unmarshal(v.Schema, &rules) != nil || rules == nil || len(rules) > 30 {
		return HGErrInvalid
	}
	for name, rule := range rules {
		if !hgName.MatchString(name) || rule.Type != "string" || rule.MaxLength < 0 || rule.MaxLength > 2048 || len(rule.Pattern) > 256 {
			return HGErrInvalid
		}
		if rule.Pattern != "" {
			if _, err := regexp.Compile("^(?:" + rule.Pattern + ")$"); err != nil {
				return HGErrInvalid
			}
		}
	}
	for _, text := range []string{v.Title, v.Body} {
		for _, match := range hgPlaceholder.FindAllStringSubmatch(text, -1) {
			if _, ok := rules[match[1]]; !ok {
				return HGErrInvalid
			}
		}
		remaining := hgPlaceholder.ReplaceAllString(text, "")
		if strings.Contains(remaining, "{{") || strings.Contains(remaining, "}}") {
			return HGErrInvalid
		}
	}
	return nil
}

// HGRender 严格校验白名单变量与输出边界；ctx 允许工作者在取消后及时停止。
func HGRender(ctx context.Context, version HGVersion, variables map[string]string) (notification.HGNotice, error) {
	if err := ctx.Err(); err != nil {
		return notification.HGNotice{}, err
	}
	if err := HGValidateVersion(version); err != nil {
		return notification.HGNotice{}, err
	}
	var rules map[string]hgRule
	if json.Unmarshal(version.Schema, &rules) != nil || len(variables) > len(rules) {
		return notification.HGNotice{}, HGErrInvalid
	}
	for name, rule := range rules {
		value, exists := variables[name]
		if rule.Required && (!exists || value == "") {
			return notification.HGNotice{}, HGErrInvalid
		}
		if !exists {
			continue
		}
		maxLength := rule.MaxLength
		if maxLength == 0 {
			maxLength = 256
		}
		if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxLength || strings.ContainsAny(value, "<>") {
			return notification.HGNotice{}, HGErrInvalid
		}
		if rule.Pattern != "" {
			pattern, _ := regexp.Compile("^(?:" + rule.Pattern + ")$")
			if !pattern.MatchString(value) {
				return notification.HGNotice{}, HGErrInvalid
			}
		}
	}
	for name := range variables {
		if _, ok := rules[name]; !ok {
			return notification.HGNotice{}, HGErrInvalid
		}
	}
	replace := func(text string) (string, error) {
		missing := false
		result := hgPlaceholder.ReplaceAllStringFunc(text, func(token string) string {
			name := token[2 : len(token)-2]
			value, ok := variables[name]
			if !ok {
				missing = true
			}
			return value
		})
		if missing {
			return "", HGErrInvalid
		}
		return result, nil
	}
	title, err := replace(version.Title)
	if err != nil {
		return notification.HGNotice{}, err
	}
	body, err := replace(version.Body)
	if err != nil {
		return notification.HGNotice{}, err
	}
	if len(title) == 0 || len(title) > 255 || len(body) == 0 || len(body) > 2048 {
		return notification.HGNotice{}, HGErrInvalid
	}
	return notification.HGNotice{Category: version.Category, Title: title, Content: body}, nil
}

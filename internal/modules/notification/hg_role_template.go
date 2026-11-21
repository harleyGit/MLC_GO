package notification

import (
	"encoding/json"
	"strings"
)

// hgValidateRolePublish 确保角色通知能够在站内入箱及实时下发；禁止发布缺少变量的草稿。
func hgValidateRolePublish(title, body string, schemaRaw, channelsRaw []byte) error {
	var schema map[string]hgVariableRule
	var channels []string
	if json.Unmarshal(schemaRaw, &schema) != nil || json.Unmarshal(channelsRaw, &channels) != nil {
		return HGErrInvalid
	}
	role, ok := schema["role_name"]
	if !ok || role.Type != "string" || !role.Required || role.MaxLength < 1 || role.MaxLength > 128 ||
		!strings.Contains(body, "{{role_name}}") || len([]rune(title)) > 200 || len([]rune(body)) > 2048 {
		return HGErrInvalid
	}
	hasInbox, hasRealtime := false, false
	for _, channel := range channels {
		hasInbox = hasInbox || channel == "in_app"
		hasRealtime = hasRealtime || channel == "realtime"
	}
	if !hasInbox || !hasRealtime {
		return HGErrInvalid
	}
	return nil
}

// HGValidateRolePublished 在消费者发送前核对已发布的角色更新模板，不信任数据库中的草稿渠道。
func HGValidateRolePublished(title, body string, schemaRaw, channelsRaw []byte) error {
	return hgValidateRolePublish(title, body, schemaRaw, channelsRaw)
}

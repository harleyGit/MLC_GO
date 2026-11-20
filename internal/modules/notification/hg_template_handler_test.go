package notification

import (
	"encoding/json"
	"testing"
)

func hgValidTemplateForm() HGTemplateForm {
	return HGTemplateForm{
		TemplateCode: "order.payment_success",
		Name:         "支付成功", Category: "transaction",
		TitleTemplate: "支付成功", BodyTemplate: "订单 {{order_no}} 支付成功",
		VariablesSchema: json.RawMessage(`{"order_no":{"type":"string","required":true,"max_length":64}}`),
		ActionType:      "order_detail",
		ActionConfig:    json.RawMessage(`{"route_name":"order_detail","params":{"orderId":"{{order_no}}"}}`),
		Channels:        json.RawMessage(`["in_app"]`),
	}
}

func TestHGValidateTemplateForm(t *testing.T) {
	form := hgValidTemplateForm()
	if err := HGValidateTemplateForm(&form, true); err != nil {
		t.Fatal(err)
	}
	if form.Priority != 2 || form.Locale != "zh-CN" {
		t.Fatalf("默认值错误: %+v", form)
	}
	for _, test := range []struct {
		name  string
		alter func(*HGTemplateForm)
	}{
		{"未声明占位符", func(f *HGTemplateForm) { f.BodyTemplate = "{{other}}" }},
		{"未闭合占位符", func(f *HGTemplateForm) { f.BodyTemplate = "{{order_no" }},
		{"外部跳转", func(f *HGTemplateForm) { f.ActionType = "https://example.com" }},
		{"路由不匹配", func(f *HGTemplateForm) { f.ActionType = "security_center" }},
		{"无站内渠道", func(f *HGTemplateForm) { f.Channels = json.RawMessage(`["realtime"]`) }},
		{"错误 JSON", func(f *HGTemplateForm) { f.VariablesSchema = json.RawMessage(`[]`) }},
		{"不可修改编码", func(f *HGTemplateForm) { f.Version = 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			item := hgValidTemplateForm()
			test.alter(&item)
			if err := HGValidateTemplateForm(&item, test.name != "不可修改编码"); err == nil {
				t.Fatal("应拒绝无效模板")
			}
		})
	}
	editing := hgValidTemplateForm()
	editing.TemplateCode = ""
	editing.Version = 3
	if err := HGValidateTemplateForm(&editing, false); err != nil {
		t.Fatalf("更新现有模板应允许: %v", err)
	}
}

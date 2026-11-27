package hgerrors

const (
	HGNotificationTemplateNotFound        Code = 1901001
	HGNotificationTemplateDisabled        Code = 1901002
	HGNotificationTemplateVariableMissing Code = 1901003
	HGNotificationTemplateConflict        Code = 1901004
	HGNotificationChannelDisabled         Code = 1902001
	HGNotificationDeliveryFailed          Code = 1902002
	HGNotificationFrequencyLimited        Code = 1902003
	HGNotificationNotFound                Code = 1903001
)

var hgNotification = []Definition{
	{HGNotificationTemplateNotFound, "NOTIFICATION_TEMPLATE_NOT_FOUND", "模板不存在", 404, false},
	{HGNotificationTemplateDisabled, "NOTIFICATION_TEMPLATE_DISABLED", "模板已停用", 409, false},
	{HGNotificationTemplateVariableMissing, "NOTIFICATION_TEMPLATE_VARIABLE_MISSING", "模板变量缺失", 400, false},
	{HGNotificationTemplateConflict, "NOTIFICATION_TEMPLATE_CONFLICT", "通知模板状态冲突", 409, false},
	{HGNotificationChannelDisabled, "NOTIFICATION_CHANNEL_DISABLED", "通知渠道不可用", 503, true},
	{HGNotificationDeliveryFailed, "NOTIFICATION_DELIVERY_FAILED", "通知投递失败", 503, true},
	{HGNotificationFrequencyLimited, "NOTIFICATION_FREQUENCY_LIMITED", "通知发送频率超限", 429, true},
	{HGNotificationNotFound, "NOTIFICATION_NOT_FOUND", "通知不存在", 404, false},
}

package hgerrors

const (
	HGIMConversationNotFound  Code = 1301001
	HGIMNotConversationMember Code = 1301002
	HGIMConversationMuted     Code = 1301003
	HGIMAccessDenied          Code = 1301004
	HGIMMessageEmpty          Code = 1302001
	HGIMMessageTooLong        Code = 1302002
	HGIMMessageDuplicated     Code = 1302003
	HGIMMessageRecalled       Code = 1302004
	HGIMMessageRecallExpired  Code = 1302005
	HGIMMessageNotFound       Code = 1302006
	HGIMStateConflict         Code = 1302007
	HGIMUserBlocked           Code = 1303001
	HGIMSendRateLimited       Code = 1303002
	HGIMAttachmentInvalid     Code = 1304001
)

var hgIm = []Definition{
	{HGIMConversationNotFound, "IM_CONVERSATION_NOT_FOUND", "会话不存在", 404, false},
	{HGIMNotConversationMember, "IM_NOT_CONVERSATION_MEMBER", "非会话成员", 403, false},
	{HGIMConversationMuted, "IM_CONVERSATION_MUTED", "当前会话禁止发言", 403, false},
	{HGIMAccessDenied, "IM_ACCESS_DENIED", "无权执行此会话操作", 403, false},
	{HGIMMessageEmpty, "IM_MESSAGE_EMPTY", "消息内容为空", 400, false},
	{HGIMMessageTooLong, "IM_MESSAGE_TOO_LONG", "消息超长", 400, false},
	{HGIMMessageDuplicated, "IM_MESSAGE_DUPLICATED", "消息请求冲突", 409, false},
	{HGIMMessageRecalled, "IM_MESSAGE_RECALLED", "消息已撤回", 409, false},
	{HGIMMessageRecallExpired, "IM_MESSAGE_RECALL_EXPIRED", "超过撤回时限", 409, false},
	{HGIMMessageNotFound, "IM_MESSAGE_NOT_FOUND", "消息不存在", 404, false},
	{HGIMStateConflict, "IM_STATE_CONFLICT", "消息或会话状态冲突", 409, false},
	{HGIMUserBlocked, "IM_USER_BLOCKED", "消息发送受到限制", 403, false},
	{HGIMSendRateLimited, "IM_SEND_RATE_LIMITED", "发消息频率超限", 429, true},
	{HGIMAttachmentInvalid, "IM_ATTACHMENT_INVALID", "附件不可用", 400, false},
}

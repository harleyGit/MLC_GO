package hgerrors

const (
	HGCommonInternalError       Code = 1000001
	HGCommonInvalidParam        Code = 1000002
	HGCommonRateLimited         Code = 1000003
	HGCommonUnavailable         Code = 1000004
	HGCommonIdempotencyConflict Code = 1000005
	HGCommonConflict            Code = 1000006
	HGCommonNotFound            Code = 1000007
	HGCommonMethodNotAllowed    Code = 1000008
	HGCommonBodyTooLarge        Code = 1000009
	HGCommonGone                Code = 1000010
	HGCommonPreconditionFailed  Code = 1000011
	HGCommonTimeout             Code = 1000012
	HGCommonBadGateway          Code = 1000013
	HGCommonGatewayTimeout      Code = 1000014
	HGCommonUnprocessable       Code = 1000015
	HGCommonForbidden           Code = 1000016
)

var hgCommon = []Definition{
	{HGCommonInternalError, "COMMON_INTERNAL_ERROR", "服务器繁忙，请稍后重试", 500, true},
	{HGCommonInvalidParam, "COMMON_INVALID_PARAM", "请求参数无效", 400, false},
	{HGCommonRateLimited, "COMMON_RATE_LIMITED", "请求过于频繁", 429, true},
	{HGCommonUnavailable, "COMMON_UNAVAILABLE", "服务暂时不可用", 503, true},
	{HGCommonIdempotencyConflict, "COMMON_IDEMPOTENCY_CONFLICT", "幂等键对应的请求内容不同", 409, false},
	{HGCommonConflict, "COMMON_CONFLICT", "当前资源状态冲突", 409, false},
	{HGCommonNotFound, "COMMON_NOT_FOUND", "请求的资源不存在", 404, false},
	{HGCommonMethodNotAllowed, "COMMON_METHOD_NOT_ALLOWED", "请求方法不允许", 405, false},
	{HGCommonBodyTooLarge, "COMMON_BODY_TOO_LARGE", "请求体过大", 413, false},
	{HGCommonGone, "COMMON_GONE", "资源已移除", 410, false},
	{HGCommonPreconditionFailed, "COMMON_PRECONDITION_FAILED", "前置条件不满足", 412, false},
	{HGCommonTimeout, "COMMON_TIMEOUT", "请求超时", 408, false},
	{HGCommonBadGateway, "COMMON_BAD_GATEWAY", "上游服务响应异常", 502, true},
	{HGCommonGatewayTimeout, "COMMON_GATEWAY_TIMEOUT", "上游服务超时", 504, true},
	{HGCommonUnprocessable, "COMMON_UNPROCESSABLE", "请求内容不符合要求", 422, false},
	{HGCommonForbidden, "COMMON_FORBIDDEN", "无权执行此操作", 403, false},
}

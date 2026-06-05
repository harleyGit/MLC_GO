/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 21:28:25
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-06-05 22:57:19
 * @FilePath: /MLC_GO/internal/handler/response/hg_err_result.go
 * @Description: 错误码定义 — 6位编码，首位分类，中间模块，末尾具体错误

大厂标准设计：6位编码，首位分类，中间模块，末尾具体错误
	段位	含义
	1xxxxx	通用请求/协议错误
	2xxxxx	成功（保留）
	3xxxxx	认证/授权相关
	4xxxxx	业务错误（按模块细分）
	5xxxxx	系统/基础设施错误
*/
package HGResponsePakcage

// ============================================================
// 类型定义
// ============================================================

type HGErrorCode int

type HGErrorResult struct {
	Code    HGErrorCode `json:"-"`
	Message string      `json:"-"`
}

type HGSuccessResult struct {
	Code    HGErrorCode `json:"-"`
	Message string      `json:"-"`
}

// ============================================================
// 错误码编码规范（6位）
//   1xxxxx — 通用请求/协议错误
//   2xxxxx — 成功（保留）
//   3xxxxx — 认证/授权相关
//   4xxxxx — 业务错误（中间两位为模块编号）
//   5xxxxx — 系统/基础设施错误
// ============================================================

const (
	// ============================================================
	// 成功
	// ============================================================
	OKCode HGErrorCode = 200

	// ============================================================
	// 1xxxxx 通用请求/协议错误
	// ============================================================
	InvalidParamCode       HGErrorCode = 100001 // 参数无效
	RequestBodyInvalidCode HGErrorCode = 100002 // 请求体无效
	RequestMethodCode      HGErrorCode = 100003 // 请求方法不允许
	RequestHeaderCode      HGErrorCode = 100004 // 请求头无效
	RequestURICode         HGErrorCode = 100005 // 请求URI无效
	RequestTimeoutCode     HGErrorCode = 100006 // 请求超时
	RequestTooLargeCode    HGErrorCode = 100007 // 请求体过大
	RateLimitCode          HGErrorCode = 100008 // 请求频率过高
	ConflictCode           HGErrorCode = 100009 // 资源冲突
	GoneCode               HGErrorCode = 100010 // 资源已删除
	PreconditionCode       HGErrorCode = 100011 // 前置条件失败

	// ============================================================
	// 3xxxxx 认证/授权相关
	// ============================================================
	UnauthorizedCode     HGErrorCode = 300001 // 未授权（未登录）
	TokenExpiredCode     HGErrorCode = 300002 // 令牌过期
	TokenInvalidCode     HGErrorCode = 300003 // 令牌无效
	ForbiddenCode        HGErrorCode = 300004 // 禁止访问（无权限）
	AccountDisabledCode  HGErrorCode = 300005 // 账号已禁用
	AccountLockedCode    HGErrorCode = 300006 // 账号已锁定
	LoginFailedCode      HGErrorCode = 300007 // 登录失败
	LogoutFailedCode     HGErrorCode = 300008 // 登出失败
	PermissionDeniedCode HGErrorCode = 300009 // 权限不足

	// ============================================================
	// 4xxxxx 业务错误
	//   401xxx — 用户模块
	//   402xxx — 订单模块
	//   403xxx — 支付模块
	//   404xxx — 认证业务
	//   405xxx — 内容/文件模块
	//   406xxx — 消息/通知模块
	//   407xxx — 系统配置模块
	// ============================================================

	// 401xxx 用户模块
	UserNotFoundCode      HGErrorCode = 401001 // 用户不存在
	UserAlreadyExistsCode HGErrorCode = 401002 // 用户已存在
	UserRegisterFailCode  HGErrorCode = 401003 // 用户注册失败
	UserUpdateFailCode    HGErrorCode = 401004 // 用户信息更新失败
	UserDeleteFailCode    HGErrorCode = 401005 // 用户删除失败
	UserListFailCode      HGErrorCode = 401006 // 获取用户列表失败
	UserProfileFailCode   HGErrorCode = 401007 // 获取用户详情失败
	UserAvatarUploadCode  HGErrorCode = 401008 // 头像上传失败

	// 402xxx 订单模块
	OrderNotFoundCode      HGErrorCode = 402001 // 订单不存在
	OrderCreateFailCode    HGErrorCode = 402002 // 订单创建失败
	OrderUpdateFailCode    HGErrorCode = 402003 // 订单更新失败
	OrderCancelFailCode    HGErrorCode = 402004 // 订单取消失败
	OrderAlreadyPaidCode   HGErrorCode = 402005 // 订单已支付
	OrderExpiredCode       HGErrorCode = 402006 // 订单已过期
	OrderStatusInvalidCode HGErrorCode = 402007 // 订单状态异常

	// 403xxx 支付模块
	PaymentNotFoundCode     HGErrorCode = 403001 // 支付记录不存在
	PaymentCreateFailCode   HGErrorCode = 403002 // 创建支付失败
	PaymentVerifyFailCode   HGErrorCode = 403003 // 支付验证失败
	PaymentTimeoutCode      HGErrorCode = 403004 // 支付超时
	InsufficientBalanceCode HGErrorCode = 403005 // 余额不足
	RefundFailCode          HGErrorCode = 403006 // 退款失败
	RefundNotAllowedCode    HGErrorCode = 403007 // 不可退款

	// 404xxx 认证业务
	ResetPasswordFailCode  HGErrorCode = 404001 // 重置密码失败
	ChangePasswordFailCode HGErrorCode = 404002 // 修改密码失败
	VerifyCodeFailCode     HGErrorCode = 404003 // 验证码错误
	VerifyCodeExpiredCode  HGErrorCode = 404004 // 验证码过期
	BindPhoneFailCode      HGErrorCode = 404005 // 绑定手机号失败
	BindEmailFailCode      HGErrorCode = 404006 // 绑定邮箱失败
	OAuthBindFailCode      HGErrorCode = 404007 // 第三方账号绑定失败

	// 405xxx 内容/文件模块
	FileNotFoundCode       HGErrorCode = 405001 // 文件不存在
	FileUploadFailCode     HGErrorCode = 405002 // 文件上传失败
	FileTooLargeCode       HGErrorCode = 405003 // 文件过大
	FileTypeNotAllowedCode HGErrorCode = 405004 // 文件类型不允许
	FileDownloadFailCode   HGErrorCode = 405005 // 文件下载失败
	VideoUploadFailCode    HGErrorCode = 405006 // 视频上传失败
	VideoTranscodeFailCode HGErrorCode = 405007 // 视频转码失败
	ImageProcessFailCode   HGErrorCode = 405008 // 图片处理失败

	// 406xxx 消息/通知模块
	NotificationSendCode HGErrorCode = 406001 // 通知发送失败
	SMSSendFailCode      HGErrorCode = 406002 // 短信发送失败
	EmailSendFailCode    HGErrorCode = 406003 // 邮件发送失败
	PushSendFailCode     HGErrorCode = 406004 // 推送发送失败

	// 407xxx 系统配置模块
	ConfigNotFoundCode   HGErrorCode = 407001 // 配置项不存在
	ConfigUpdateFailCode HGErrorCode = 407002 // 配置更新失败
	DictNotFoundCode     HGErrorCode = 407003 // 字典项不存在

	// ============================================================
	// 5xxxxx 系统/基础设施错误
	// ============================================================
	InternalErrorCode      HGErrorCode = 500001 // 系统内部错误
	DatabaseErrorCode      HGErrorCode = 500002 // 数据库错误
	CacheErrorCode         HGErrorCode = 500003 // 缓存错误
	MQErrorCode            HGErrorCode = 500004 // 消息队列错误
	ThirdPartyErrorCode    HGErrorCode = 500005 // 第三方服务错误
	ConfigErrorCode        HGErrorCode = 500006 // 系统配置错误
	ServiceUnavailableCode HGErrorCode = 500007 // 服务不可用
	ServiceDegradedCode    HGErrorCode = 500008 // 服务已降级
	CircuitBreakerCode     HGErrorCode = 500009 // 熔断触发
	NetworkErrorCode       HGErrorCode = 500010 // 网络错误
	SerializationCode      HGErrorCode = 500011 // 序列化/反序列化错误
)

// ============================================================
// 预定义结果变量
// ============================================================

var (
	// 成功
	HGSuccess = HGSuccessResult{OKCode, "Success💯"}

	// ----------------------------------------------------------
	// 1xxxxx 通用请求/协议错误
	// ----------------------------------------------------------
	InvalidParam       = HGErrorResult{InvalidParamCode, "参数无效"}
	RequestBodyInvalid = HGErrorResult{RequestBodyInvalidCode, "请求体无效"}
	RequestMethod      = HGErrorResult{RequestMethodCode, "请求方法不允许"}
	RequestHeader      = HGErrorResult{RequestHeaderCode, "请求头无效"}
	RequestURI         = HGErrorResult{RequestURICode, "请求URI无效"}
	RequestTimeout     = HGErrorResult{RequestTimeoutCode, "请求超时"}
	RequestTooLarge    = HGErrorResult{RequestTooLargeCode, "请求体过大"}
	RateLimit          = HGErrorResult{RateLimitCode, "请求频率过高，请稍后重试"}
	Conflict           = HGErrorResult{ConflictCode, "资源冲突"}
	Gone               = HGErrorResult{GoneCode, "资源已删除"}
	Precondition       = HGErrorResult{PreconditionCode, "前置条件失败"}

	// ----------------------------------------------------------
	// 3xxxxx 认证/授权相关
	// ----------------------------------------------------------
	Unauthorized     = HGErrorResult{UnauthorizedCode, "未授权，请先登录"}
	TokenExpired     = HGErrorResult{TokenExpiredCode, "令牌已过期，请重新登录"}
	TokenInvalid     = HGErrorResult{TokenInvalidCode, "令牌无效"}
	Forbidden        = HGErrorResult{ForbiddenCode, "禁止访问，权限不足"}
	AccountDisabled  = HGErrorResult{AccountDisabledCode, "账号已禁用"}
	AccountLocked    = HGErrorResult{AccountLockedCode, "账号已锁定"}
	LoginFailed      = HGErrorResult{LoginFailedCode, "登录失败"}
	LogoutFailed     = HGErrorResult{LogoutFailedCode, "登出失败"}
	PermissionDenied = HGErrorResult{PermissionDeniedCode, "权限不足"}

	// ----------------------------------------------------------
	// 401xxx 用户模块
	// ----------------------------------------------------------
	UserNotFound      = HGErrorResult{UserNotFoundCode, "用户不存在"}
	UserAlreadyExists = HGErrorResult{UserAlreadyExistsCode, "用户已存在"}
	UserRegisterFailed = HGErrorResult{UserRegisterFailCode, "用户注册失败"}
	UserUpdateFailed  = HGErrorResult{UserUpdateFailCode, "用户信息更新失败"}
	UserDeleteFailed  = HGErrorResult{UserDeleteFailCode, "用户删除失败"}
	UserListFailed    = HGErrorResult{UserListFailCode, "获取用户列表失败"}
	UserProfileFailed = HGErrorResult{UserProfileFailCode, "获取用户详情失败"}
	UserAvatarUpload  = HGErrorResult{UserAvatarUploadCode, "头像上传失败"}

	// ----------------------------------------------------------
	// 402xxx 订单模块
	// ----------------------------------------------------------
	OrderNotFound      = HGErrorResult{OrderNotFoundCode, "订单不存在"}
	OrderCreateFailed  = HGErrorResult{OrderCreateFailCode, "订单创建失败"}
	OrderUpdateFailed  = HGErrorResult{OrderUpdateFailCode, "订单更新失败"}
	OrderCancelFailed  = HGErrorResult{OrderCancelFailCode, "订单取消失败"}
	OrderAlreadyPaid   = HGErrorResult{OrderAlreadyPaidCode, "订单已支付，不可重复操作"}
	OrderExpired       = HGErrorResult{OrderExpiredCode, "订单已过期"}
	OrderStatusInvalid = HGErrorResult{OrderStatusInvalidCode, "订单状态异常"}

	// ----------------------------------------------------------
	// 403xxx 支付模块
	// ----------------------------------------------------------
	PaymentNotFound     = HGErrorResult{PaymentNotFoundCode, "支付记录不存在"}
	PaymentCreateFailed = HGErrorResult{PaymentCreateFailCode, "创建支付失败"}
	PaymentVerifyFailed = HGErrorResult{PaymentVerifyFailCode, "支付验证失败"}
	PaymentTimeout      = HGErrorResult{PaymentTimeoutCode, "支付超时"}
	InsufficientBalance = HGErrorResult{InsufficientBalanceCode, "余额不足"}
	RefundFailed        = HGErrorResult{RefundFailCode, "退款失败"}
	RefundNotAllowed    = HGErrorResult{RefundNotAllowedCode, "该订单不可退款"}

	// ----------------------------------------------------------
	// 404xxx 认证业务
	// ----------------------------------------------------------
	ResetPasswordFailed  = HGErrorResult{ResetPasswordFailCode, "重置密码失败"}
	ChangePasswordFailed = HGErrorResult{ChangePasswordFailCode, "修改密码失败"}
	VerifyCodeFailed     = HGErrorResult{VerifyCodeFailCode, "验证码错误"}
	VerifyCodeExpired    = HGErrorResult{VerifyCodeExpiredCode, "验证码已过期"}
	BindPhoneFailed      = HGErrorResult{BindPhoneFailCode, "绑定手机号失败"}
	BindEmailFailed      = HGErrorResult{BindEmailFailCode, "绑定邮箱失败"}
	OAuthBindFailed      = HGErrorResult{OAuthBindFailCode, "第三方账号绑定失败"}

	// ----------------------------------------------------------
	// 405xxx 内容/文件模块
	// ----------------------------------------------------------
	FileNotFound       = HGErrorResult{FileNotFoundCode, "文件不存在"}
	FileUploadFailed   = HGErrorResult{FileUploadFailCode, "文件上传失败"}
	FileTooLarge       = HGErrorResult{FileTooLargeCode, "文件过大"}
	FileTypeNotAllowed = HGErrorResult{FileTypeNotAllowedCode, "文件类型不允许"}
	FileDownloadFailed = HGErrorResult{FileDownloadFailCode, "文件下载失败"}
	VideoUploadFailed  = HGErrorResult{VideoUploadFailCode, "视频上传失败"}
	VideoTranscodeFailed = HGErrorResult{VideoTranscodeFailCode, "视频转码失败"}
	ImageProcessFailed = HGErrorResult{ImageProcessFailCode, "图片处理失败"}

	// ----------------------------------------------------------
	// 406xxx 消息/通知模块
	// ----------------------------------------------------------
	NotificationSendFailed = HGErrorResult{NotificationSendCode, "通知发送失败"}
	SMSSendFailed          = HGErrorResult{SMSSendFailCode, "短信发送失败"}
	EmailSendFailed        = HGErrorResult{EmailSendFailCode, "邮件发送失败"}
	PushSendFailed         = HGErrorResult{PushSendFailCode, "推送发送失败"}

	// ----------------------------------------------------------
	// 407xxx 系统配置模块
	// ----------------------------------------------------------
	ConfigNotFound    = HGErrorResult{ConfigNotFoundCode, "配置项不存在"}
	ConfigUpdateFailed = HGErrorResult{ConfigUpdateFailCode, "配置更新失败"}
	DictNotFound      = HGErrorResult{DictNotFoundCode, "字典项不存在"}

	// ----------------------------------------------------------
	// 5xxxxx 系统/基础设施错误
	// ----------------------------------------------------------
	InternalError      = HGErrorResult{InternalErrorCode, "系统内部错误"}
	DatabaseError      = HGErrorResult{DatabaseErrorCode, "数据库错误"}
	CacheError         = HGErrorResult{CacheErrorCode, "缓存错误"}
	MQError            = HGErrorResult{MQErrorCode, "消息队列错误"}
	ThirdPartyError    = HGErrorResult{ThirdPartyErrorCode, "第三方服务错误"}
	ConfigError        = HGErrorResult{ConfigErrorCode, "系统配置错误"}
	ServiceUnavailable = HGErrorResult{ServiceUnavailableCode, "服务暂不可用，请稍后重试"}
	ServiceDegraded    = HGErrorResult{ServiceDegradedCode, "服务已降级"}
	CircuitBreaker     = HGErrorResult{CircuitBreakerCode, "熔断触发，请稍后重试"}
	NetworkError       = HGErrorResult{NetworkErrorCode, "网络错误"}
	SerializationError = HGErrorResult{SerializationCode, "数据序列化错误"}
)

// ============================================================
// 工厂方法
// ============================================================

func NewErrResult(code HGErrorCode, msg string) *HGErrorResult {
	return &HGErrorResult{Code: code, Message: msg}
}

func (e *HGErrorResult) ErrMSG() string {
	return e.Message
}

func (e HGErrorResult) ResponseCode() HGErrorCode {
	return e.Code
}

func (e HGErrorResult) ResponseMessage() string {
	return e.Message
}

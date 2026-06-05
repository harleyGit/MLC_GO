package UserHandlerPackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	PkGDevicePackage "MLC_GO/internal/pkg/device"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"net/http"
)

// RefreshTokenRequest 是刷新 Token 的 HTTP 请求体。
// refreshToken 使用驼峰命名是为了兼容当前前端字段，不在重构中改变外部协议。
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// RefreshTokenResponse 是刷新 Token 的 HTTP 响应体。
// token 字段保持现有协议命名，避免客户端升级成本。
type RefreshTokenResponse struct {
	AccessToken  string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}

// ClickCaptchaVerifyBody 是验证点选验证码的 HTTP 请求体。
type ClickCaptchaVerifyBody struct {
	CaptchaID string                                 `json:"captchaId"`
	Points    []UserServicePackage.ClickCaptchaPoint `json:"points"`
}

// GetClickCaptcha 获取点选验证码图片。
// 返回验证码 ID、图片 Base64 数据和需要点选的字符序列。
func (h *HGUserHandler) GetClickCaptcha(w http.ResponseWriter, r *http.Request) {
	if h.clickCaptchaSvc == nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: "验证码服务未初始化"})
		return
	}

	resp, err := h.clickCaptchaSvc.GenerateCaptcha(r.Context())
	if err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: "生成验证码失败: "+err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// VerifyClickCaptcha 验证点选验证码。
// 验证通过后返回 verifyToken，用于后续发送短信/邮箱验证码。
func (h *HGUserHandler) VerifyClickCaptcha(w http.ResponseWriter, r *http.Request) {
	if h.clickCaptchaSvc == nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: "验证码服务未初始化"})
		return
	}

	var req ClickCaptchaVerifyBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "请求体格式错误"})
		return
	}

	if req.CaptchaID == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "captchaId 不能为空"})
		return
	}

	if len(req.Points) == 0 {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "点选坐标不能为空"})
		return
	}

	verifyReq := &UserServicePackage.ClickCaptchaVerifyRequest{
		CaptchaID: req.CaptchaID,
		Points:    req.Points,
	}

	resp, err := h.clickCaptchaSvc.VerifyCaptcha(r.Context(), verifyReq)
	if err != nil {
		if err == UserServicePackage.ErrCaptchaNotFound {
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "验证码已过期，请重新获取"})
			return
		}
		if err == UserServicePackage.ErrCaptchaInvalid {
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "验证码验证失败，请重新点选"})
			return
		}
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: "验证失败: "+err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// RegisterHandlerV3 处理用户注册请求。
// handler 只解析 JSON 并调用 service.Register，验证码校验、密码加密、落库和缓存清理由 service 完成。
func (h *HGUserHandler) RegisterHandlerV3(w http.ResponseWriter, r *http.Request) {
	var req UserDtoPackage.RegisterReqModel
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "请求体格式错误"})
		return
	}

	if err := h.svc.Register(r.Context(), req); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.UserRegisterFailCode, Message: "注册失败: "+err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, req)
}

// SendCode 发送登录/注册验证码。
// 生成和写入 Redis 属于业务流程，由 service 完成；短信发送由 handler 调用注入的短信端口，便于替换供应商。
func (h *HGUserHandler) SendCode(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "缺少 phone 参数"})
		return
	}

	// verifyToken 是点选验证码验证通过后生成的一次性凭证。
	// 发送短信验证码是高成本、易被刷的外部 I/O 行为，必须先验证该 token，避免攻击者直接刷 send_code。
	// token 校验通过后会在 service 中删除，因此同一个点选验证码结果不能重复换取多个短信验证码。
	verifyToken := r.URL.Query().Get("verifyToken")
	if ok := h.validateClickCaptchaToken(w, r, verifyToken); !ok {
		return
	}

	code, err := h.svc.SendCode(r.Context(), phone)
	if err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: err.Error()})
		return
	}

	if err := h.smsSender.Send(phone, code); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: "发送短信失败"})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, map[string]string{"phone": phone, "message": "验证码已发送", "verifyCode": code})
}

// SendEmailCode 发送邮箱验证码。
// 高并发设计：使用独立 Redis key 前缀隔离邮箱验证码，避免与手机验证码冲突；验证码一次性消费。
func (h *HGUserHandler) SendEmailCode(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "缺少 email 参数"})
		return
	}

	// 邮箱验证码同样走点选验证码前置校验，避免绕过弹窗直接刷邮件发送接口。
	// 这里和 SendCode 共用 validateClickCaptchaToken，保证手机和邮箱验证码入口安全语义一致。
	verifyToken := r.URL.Query().Get("verifyToken")
	if ok := h.validateClickCaptchaToken(w, r, verifyToken); !ok {
		return
	}

	code, err := h.svc.SendEmailCode(r.Context(), email)
	if err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: err.Error()})
		return
	}

	// TODO: 调用邮件发送服务发送验证码
	// 这里暂时返回验证码，实际生产环境需要调用邮件服务
	HGResponsePakcage.SuccessResult(w, r, map[string]string{"email": email, "message": "验证码已发送", "verifyCode": code})
}

// validateClickCaptchaToken 校验点选验证码通过后生成的一次性 token。
// 返回 true 表示当前请求可以继续发送短信/邮箱验证码；返回 false 时本函数已经写入 HTTP 响应，调用方应立即 return。
// 这个 helper 放在 handler 层，是因为它负责 HTTP 参数校验和 HTTP 错误响应；真正的 token 查询、删除和 TTL 语义仍由 service 层负责。
func (h *HGUserHandler) validateClickCaptchaToken(w http.ResponseWriter, r *http.Request, verifyToken string) bool {
	if h.clickCaptchaSvc == nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: "验证码服务未初始化"})
		return false
	}

	// ValidateVerifyToken 会读取 Redis 中 auth:click_captcha_token:<token>，存在则删除并返回 true。
	// 删除动作使 token 具备一次性消费语义，避免并发或重复点击导致同一 token 被多次使用。
	valid, err := h.clickCaptchaSvc.ValidateVerifyToken(r.Context(), verifyToken)
	if err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: "验证码校验失败: "+err.Error()})
		return false
	}
	if !valid {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "请先完成点选验证码"})
		return false
	}

	return true
}

// RegisterWithEmail 处理邮箱注册请求。
// 高并发设计：复用现有注册流程，邮箱验证码独立 Redis key，避免与手机验证码冲突。
func (h *HGUserHandler) RegisterWithEmail(w http.ResponseWriter, r *http.Request) {
	var req UserDtoPackage.RegisterReqModel
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "请求体格式错误"})
		return
	}

	if req.Email == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "邮箱不能为空"})
		return
	}

	if req.Code == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "验证码不能为空"})
		return
	}

	if req.Password == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "密码不能为空"})
		return
	}

	// 验证邮箱验证码
	if err := h.svc.VerifyEmailCode(r.Context(), req.Email, req.Code); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "验证码错误或已过期"})
		return
	}

	// 调用邮箱注册服务
	if err := h.svc.RegisterWithEmail(r.Context(), req); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.UserRegisterFailCode, Message: "注册失败: "+err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, req)
}

// Login 处理用户登录，支持验证码和密码两种方式。
// 设备指纹在 HTTP 层提取，认证规则和 token 签发统一交给 service，避免 handler 写业务判断。
func (h *HGUserHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UserServicePackage.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "JSON 解析失败: "+err.Error()})
		return
	}
	req.Device = PkGDevicePackage.Fingerprint(r)

	resp, err := h.svc.Login(r.Context(), &req)
	if err != nil {
		switch err {
		case UserServicePackage.ErrUserNotFound:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.UserNotFoundCode, Message: "用户不存在"})
		case UserServicePackage.ErrPasswordIncorrect:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "密码不正确"})
		case UserServicePackage.ErrCodeInvalid:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "验证码无效或已过期"})
		case UserServicePackage.ErrPhoneOrEmailRequired:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "手机号或邮箱必填"})
		default:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: "登录失败: "+err.Error()})
		}
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// SendResetPasswordCode 发送忘记密码验证码。
// 使用独立验证码通道，与注册/登录验证码隔离，避免互相覆盖。
func (h *HGUserHandler) SendResetPasswordCode(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "缺少 phone 参数"})
		return
	}

	code, err := h.svc.SendResetPasswordCode(r.Context(), phone)
	if err != nil {
		if err == UserServicePackage.ErrUserNotFound {
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.UserNotFoundCode, Message: "用户不存在"})
			return
		}
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: err.Error()})
		return
	}

	if err := h.smsSender.Send(phone, code); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: "发送短信失败"})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, map[string]string{"phone": phone, "message": "验证码已发送", "verifyCode": code})
}

// ResetPassword 处理忘记密码请求。
// 通过手机验证码验证用户身份后重置密码，handler 只解析 JSON 并调用 service。
func (h *HGUserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UserDtoPackage.ResetPasswordReqModel
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "JSON 解析失败: "+err.Error()})
		return
	}

	if err := h.svc.ResetPassword(r.Context(), &req); err != nil {
		switch err {
		case UserServicePackage.ErrUserNotFound:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.UserNotFoundCode, Message: "用户不存在"})
		case UserServicePackage.ErrCodeInvalid:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "验证码无效或已过期"})
		default:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.ResetPasswordFailCode, Message: "重置密码失败: "+err.Error()})
		}
		return
	}

	HGResponsePakcage.SuccessResult(w, r, map[string]string{"message": "密码重置成功"})
}

// RefreshToken 刷新 Access Token。
// token 校验、Redis 状态检查和重放防护属于认证 service，不允许 handler 直接访问 Redis 客户端。
func (h *HGUserHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "JSON 解析失败: "+err.Error()})
		return
	}
	if req.RefreshToken == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParamCode, Message: "refreshToken 不能为空"})
		return
	}

	tokenPair, err := h.tokenService.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "刷新 Token 失败: "+err.Error())
		return
	}

	HGResponsePakcage.SuccessResult(w, r, RefreshTokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	})
}

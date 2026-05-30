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

// RegisterHandlerV3 处理用户注册请求。
// handler 只解析 JSON 并调用 service.Register，验证码校验、密码加密、落库和缓存清理由 service 完成。
func (h *HGUserHandler) RegisterHandlerV3(w http.ResponseWriter, r *http.Request) {
	var req UserDtoPackage.RegisterReqModel
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "请求体格式错误")
		return
	}

	if err := h.svc.Register(r.Context(), req); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserRegisterFail, "注册失败: "+err.Error())
		return
	}

	HGResponsePakcage.SuccessResult(w, r, req)
}

// SendCode 发送登录/注册验证码。
// 生成和写入 Redis 属于业务流程，由 service 完成；短信发送由 handler 调用注入的短信端口，便于替换供应商。
func (h *HGUserHandler) SendCode(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "缺少 phone 参数")
		return
	}

	code, err := h.svc.SendCode(r.Context(), phone)
	if err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, err.Error())
		return
	}

	if err := h.smsSender.Send(phone, code); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "发送短信失败")
		return
	}

	HGResponsePakcage.SuccessResult(w, r, map[string]string{"phone": phone, "message": "验证码已发送", "verifyCode": code})
}

// SendEmailCode 发送邮箱验证码。
// 高并发设计：使用独立 Redis key 前缀隔离邮箱验证码，避免与手机验证码冲突；验证码一次性消费。
func (h *HGUserHandler) SendEmailCode(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "缺少 email 参数")
		return
	}

	code, err := h.svc.SendEmailCode(r.Context(), email)
	if err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, err.Error())
		return
	}

	// TODO: 调用邮件发送服务发送验证码
	// 这里暂时返回验证码，实际生产环境需要调用邮件服务
	HGResponsePakcage.SuccessResult(w, r, map[string]string{"email": email, "message": "验证码已发送", "verifyCode": code})
}

// RegisterWithEmail 处理邮箱注册请求。
// 高并发设计：复用现有注册流程，邮箱验证码独立 Redis key，避免与手机验证码冲突。
func (h *HGUserHandler) RegisterWithEmail(w http.ResponseWriter, r *http.Request) {
	var req UserDtoPackage.RegisterReqModel
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "请求体格式错误")
		return
	}

	if req.Email == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "邮箱不能为空")
		return
	}

	if req.Code == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "验证码不能为空")
		return
	}

	if req.Password == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "密码不能为空")
		return
	}

	// 验证邮箱验证码
	if err := h.svc.VerifyEmailCode(r.Context(), req.Email, req.Code); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "验证码错误或已过期")
		return
	}

	// 调用邮箱注册服务
	if err := h.svc.RegisterWithEmail(r.Context(), req); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserRegisterFail, "注册失败: "+err.Error())
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
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "JSON 解析失败: "+err.Error())
		return
	}
	req.Device = PkGDevicePackage.Fingerprint(r)

	resp, err := h.svc.Login(r.Context(), &req)
	if err != nil {
		switch err {
		case UserServicePackage.ErrUserNotFound:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserNotFoundCode, "用户不存在")
		case UserServicePackage.ErrPasswordIncorrect:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "密码不正确")
		case UserServicePackage.ErrCodeInvalid:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "验证码无效或已过期")
		case UserServicePackage.ErrPhoneOrEmailRequired:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "手机号或邮箱必填")
		default:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "登录失败: "+err.Error())
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
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "缺少 phone 参数")
		return
	}

	code, err := h.svc.SendResetPasswordCode(r.Context(), phone)
	if err != nil {
		if err == UserServicePackage.ErrUserNotFound {
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserNotFoundCode, "用户不存在")
			return
		}
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, err.Error())
		return
	}

	if err := h.smsSender.Send(phone, code); err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "发送短信失败")
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
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "JSON 解析失败: "+err.Error())
		return
	}

	if err := h.svc.ResetPassword(r.Context(), &req); err != nil {
		switch err {
		case UserServicePackage.ErrUserNotFound:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserNotFoundCode, "用户不存在")
		case UserServicePackage.ErrCodeInvalid:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "验证码无效或已过期")
		default:
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.ResetPasswordFailCode, "重置密码失败: "+err.Error())
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
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "JSON 解析失败: "+err.Error())
		return
	}
	if req.RefreshToken == "" {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "refreshToken 不能为空")
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

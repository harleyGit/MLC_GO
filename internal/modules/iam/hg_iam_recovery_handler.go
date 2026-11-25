package iam

import (
	UserJWT "MLC_GO/internal/modules/user/middleware"
	UserService "MLC_GO/internal/modules/user/service"
	Mail "MLC_GO/internal/pkg/email"
	HGResponsePackage "MLC_GO/internal/response"
	"errors"
	"net/http"
)

type hgRecoverySendRequest struct {
	Email string `json:"email"` // 待验证的新恢复邮箱。
}

type hgRecoveryBindRequest struct {
	Email     string `json:"email"`     // 待绑定的新邮箱，不改变登录邮箱。
	EmailCode string `json:"emailCode"` // 五分钟有效的一次性邮件码。
	MFACode   string `json:"mfaCode"`   // 绑定新邮箱时仍要求已有因子的当前动态码。
}

type hgRecoveryResetRequest struct {
	Password  string `json:"password"`  // 丢失验证器时须复核本人登录密码。
	EmailCode string `json:"emailCode"` // 由已验证的邮箱接收的邮件码。
}

func hgRecoveryHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := http.StatusConflict, HGResponsePackage.Conflict.Code
	switch {
	case errors.Is(err, hgErrRecoveryDisabled):
		status, code = http.StatusServiceUnavailable, HGResponsePackage.InternalError.Code
	case errors.Is(err, Mail.HGErrUnavailable):
		status, code = http.StatusServiceUnavailable, HGResponsePackage.EmailSendFailCode
	case errors.Is(err, Mail.HGErrRateLimited):
		status, code = http.StatusTooManyRequests, HGResponsePackage.EmailSendFailCode
	case errors.Is(err, hgErrRecoveryCode), errors.Is(err, hgErrRecoveryPassword), errors.Is(err, hgErrRecoveryFactor):
		status, code = http.StatusForbidden, HGResponsePackage.Forbidden.Code
	case errors.Is(err, hgErrRecoveryEmail), errors.Is(err, hgErrRecoveryMFA), errors.Is(err, hgErrRecoveryCooldown),
		errors.Is(err, hgErrRecoveryTaken), errors.Is(err, hgErrRecoveryChanged):
	default:
		hgHTTPError(w, r, err)
		return
	}
	w.WriteHeader(status)
	HGResponsePackage.FailResult[string](w, r, HGResponsePackage.HGErrorResult{Code: code, Message: err.Error()})
}

// HGOwnMFAEnroll 所有用户仅能登记本人的 MFA，原 IAM 管理员路由不变。
func (h *HGHandler) HGOwnMFAEnroll(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGMFARequest
	if !hgDecode(w, r, &req) {
		return
	}
	secret, uri, err := h.service.HGEnrollOwnMFA(r.Context(), actor, req.Password)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	HGResponsePackage.SuccessResult(w, r, map[string]string{"secret": secret, "otpauthUri": uri})
}

// HGOwnMFAConfirm 启用本人已登记的 TOTP，不接收第三方用户身份。
func (h *HGHandler) HGOwnMFAConfirm(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGMFARequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGConfirmOwnMFA(r.Context(), actor, req.Code); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"enabled": true})
}

// HGOwnRecoveryStatus 返回已验证邮箱掩码、冷却期与 MFA 启用状态。
func (h *HGHandler) HGOwnRecoveryStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	status, err := h.service.HGOwnRecoveryStatus(r.Context(), actor)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, status)
}

// HGOwnRecoverySendBindCode 发送邮箱码；邮件发送失败不声称成功。
func (h *HGHandler) HGOwnRecoverySendBindCode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req hgRecoverySendRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGSendOwnRecoveryBindCode(r.Context(), actor, req.Email); err != nil {
		hgRecoveryHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"sent": true})
}

// HGOwnRecoverySendResetCode 只能发给本人已验证的恢复邮箱。
func (h *HGHandler) HGOwnRecoverySendResetCode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req struct{}
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGSendOwnRecoveryResetCode(r.Context(), actor); err != nil {
		hgRecoveryHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"sent": true})
}

// HGOwnRecoveryBind 仅邮件码加已有 MFA 均通过后才确认邮箱已验证。
func (h *HGHandler) HGOwnRecoveryBind(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req hgRecoveryBindRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGBindOwnRecoveryEmail(r.Context(), actor, req.Email, req.EmailCode, req.MFACode); err != nil {
		hgRecoveryHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"emailVerified": true})
}

// HGOwnRecoveryReset 消费密码与邮箱码后重置 MFA，旧会话必须重新登录。
func (h *HGHandler) HGOwnRecoveryReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req hgRecoveryResetRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGResetOwnMFA(r.Context(), actor, req.Password, req.EmailCode); err != nil {
		hgRecoveryHTTPError(w, r, err)
		return
	}
	if claims, ok := r.Context().Value(UserJWT.UserIDKey).(*UserService.HGClaims); ok && claims != nil {
		h.service.hgDisconnectRecoveredSessions(r.Context(), actor, claims.SessionID)
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"reloginRequired": true})
}

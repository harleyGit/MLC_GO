package UserHandlerPackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

// UpdateSecurity 处理账号安全信息修改，支持 QQ、密码、手机、邮箱、微信号单字段或多字段更新。
// 用户身份优先复用 JWT 中间件写入的 claims，避免 handler 重复解析 token。
func (h *HGUserHandler) UpdateSecurity(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUpdateUserID(r)
	if err != nil {
		HGResponsePakcage.HGFailStatus(w, r, http.StatusBadRequest, HGResponsePakcage.HGSafeLegacyResult(HGResponsePakcage.InvalidParam.Code))
		return
	}

	var req UserDtoPackage.HGUpdateUserSecurityReqDTO
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		HGResponsePakcage.HGFailStatus(w, r, http.StatusBadRequest, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return
	}

	actorID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		HGResponsePakcage.FailTokenInvalid(w, r, "登录状态无效")
		return
	}
	// 目录写权限只允许管理员代改联系方式；不得借此重置他人密码或绑定第三方账号。
	if actorID != userID && (req.Password != nil || req.QQ != nil || req.Wechat != nil || (req.Phone == nil && req.Email == nil)) {
		HGResponsePakcage.HGFailStatus(w, r, http.StatusBadRequest, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "代改账号安全信息仅支持手机号或邮箱"})
		return
	}
	resp, err := h.svc.UpdateSecurity(r.Context(), userID, &req, actorID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			HGResponsePakcage.HGFailStatus(w, r, http.StatusNotFound, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.UserNotFound.Code, Message: "用户不存在"})
		case errors.Is(err, UserServicePackage.ErrSecurityNoField),
			errors.Is(err, UserServicePackage.ErrSecurityFieldEmpty):
			HGResponsePakcage.HGFailStatus(w, r, http.StatusBadRequest, HGResponsePakcage.HGSafeLegacyResult(HGResponsePakcage.InvalidParam.Code))
		case errors.Is(err, UserServicePackage.ErrSecurityDuplicate):
			HGResponsePakcage.HGFailStatus(w, r, http.StatusConflict, HGResponsePakcage.HGSafeLegacyResult(HGResponsePakcage.InvalidParam.Code))
		default:
			HGResponsePakcage.HGFailStatus(w, r, http.StatusInternalServerError, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: "更新账号安全信息失败"})
		}
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// GetSecurityInfo 返回当前用户账号安全表完整字段。
func (h *HGUserHandler) GetSecurityInfo(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUpdateUserID(r)
	if err != nil {
		HGResponsePakcage.HGFailStatus(w, r, http.StatusBadRequest, HGResponsePakcage.HGSafeLegacyResult(HGResponsePakcage.InvalidParam.Code))
		return
	}

	resp, err := h.svc.GetSecurityInfo(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			HGResponsePakcage.HGFailStatus(w, r, http.StatusNotFound, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.UserNotFound.Code, Message: "账号安全信息不存在"})
		default:
			HGResponsePakcage.HGFailStatus(w, r, http.StatusInternalServerError, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: "获取账号安全信息失败"})
		}
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

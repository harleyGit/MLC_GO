package UserHandlerPackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserServicePackage "MLC_GO/internal/modules/user/service"
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
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
		return
	}

	var req UserDtoPackage.HGUpdateUserSecurityReqDTO
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "请求体格式错误")
		return
	}

	resp, err := h.svc.UpdateSecurity(r.Context(), userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			w.WriteHeader(http.StatusNotFound)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserNotFoundCode, "用户不存在")
		case errors.Is(err, UserServicePackage.ErrSecurityNoField),
			errors.Is(err, UserServicePackage.ErrSecurityFieldEmpty):
			w.WriteHeader(http.StatusBadRequest)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
		case errors.Is(err, UserServicePackage.ErrSecurityDuplicate):
			w.WriteHeader(http.StatusConflict)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
		default:
			w.WriteHeader(http.StatusInternalServerError)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "更新账号安全信息失败")
		}
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// GetSecurityInfo 返回当前用户账号安全表完整字段。
func (h *HGUserHandler) GetSecurityInfo(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUpdateUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
		return
	}

	resp, err := h.svc.GetSecurityInfo(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			w.WriteHeader(http.StatusNotFound)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserNotFoundCode, "账号安全信息不存在")
		default:
			w.WriteHeader(http.StatusInternalServerError)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "获取账号安全信息失败")
		}
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

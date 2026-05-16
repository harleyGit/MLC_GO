package UserHandlerPackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"MLC_GO/internal/pkg/logHG"
	HGResponsePakcage "MLC_GO/internal/response"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// CreateUser 是内部创建用户入口，主要用于兼容已有测试或后台调用。
// 对外注册应优先走 RegisterHandlerV3，因为注册流程包含验证码校验和缓存清理。
func (h *HGUserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var d UserDtoPackage.HGCreateUserDTO
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.svc.CreateUser(r.Context(), &d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// GetUserList 获取用户列表，使用 cursor 分页避免大 offset 深分页扫描。
// 兼容旧 pageNum 参数，但新调用方应使用响应中的 nextCursor 继续翻页。
func (h *HGUserHandler) GetUserList(w http.ResponseWriter, r *http.Request) {
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	page, _ := strconv.Atoi(r.URL.Query().Get("pageNum"))
	size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if size <= 0 || size > 1000 {
		size = 20
	}
	if cursor <= 0 && page <= 1 {
		cursor = 0
	}

	resp, err := h.svc.GetUserList(r.Context(), cursor, size)
	if err != nil {
		HGResponsePakcage.FailResult[error](w, r, HGResponsePakcage.UserListFailCode, err.Error())
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// PathUser 按业务 user_id 局部更新用户基础信息。
// 该方法保留现有行为用于兼容旧路由；新资料更新建议使用 UpdateProfile。
func (h *HGUserHandler) PathUser(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "缺少 user_id 参数")
		return
	}

	var d UserDtoPackage.HGCreateUserDTO
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := h.svc.PathUser(r.Context(), userID, &d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(user)
}

// UpdateProfile 处理用户资料更新，支持单字段或多字段更新。
// handler 负责把 service 的业务错误映射成 HTTP 状态码和统一响应结构。
func (h *HGUserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUpdateUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
		return
	}

	var req UserDtoPackage.HGUpdateUserProfileReqDTO
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "请求体格式错误")
		return
	}

	resp, err := h.svc.UpdateProfile(r.Context(), userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			w.WriteHeader(http.StatusNotFound)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserNotFoundCode, "用户不存在")
			return
		case errors.Is(err, UserServicePackage.ErrProfileNoField),
			errors.Is(err, UserServicePackage.ErrProfileGenderInvalid),
			errors.Is(err, UserServicePackage.ErrProfileBirthDateInvalid):
			w.WriteHeader(http.StatusBadRequest)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "更新用户资料失败")
			return
		}
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// Profile 返回当前登录用户资料。
// 用户身份由 JWT 中间件提前解析并写入 context，handler 不重复解析 token。
func (h *HGUserHandler) Profile(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(UserJWTMiddlewarePackage.UserIDKey).(*UserServicePackage.HGClaims)
	if !ok {
		logHG.ErrFInfo("用户信息Profile error: %v", ok)
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	userDTO, err := h.svc.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		logHG.ErrFInfo("用户信息Profile error: %v", err)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UserNotFoundCode, "用户不存在"+err.Error())
		return
	}

	HGResponsePakcage.SuccessResult(w, r, userDTO)
}

// parseUpdateUserID 解析资料更新目标 user_id，优先读取 query 参数，缺失时尝试从 JWT claims 获取。
// 这里集中处理 user_id 来源，避免各个 handler 重复读取 query/context 导致行为不一致。
func parseUpdateUserID(r *http.Request) (string, error) {
	userIDText := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userIDText == "" {
		claims, ok := r.Context().Value(UserJWTMiddlewarePackage.UserIDKey).(*UserServicePackage.HGClaims)
		if ok && claims != nil {
			userIDText = strings.TrimSpace(claims.UserID)
		}
	}
	if userIDText == "" {
		return "", errors.New("缺少 user_id 参数")
	}

	return userIDText, nil
}

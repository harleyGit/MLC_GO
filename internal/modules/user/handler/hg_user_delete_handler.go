package UserHandlerPackage

import (
	userrepository "MLC_GO/internal/modules/user/repository"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePackage "MLC_GO/internal/response"
	"errors"
	"net/http"
	"strings"
)

// HGDeleteUnusedUser 只允许具有用户目录写权限的管理员删除其他用户的无关联账号。
// 权限由路由层强制验证；此处额外禁止删除本人及缺失目标 ID。
func (h *HGUserHandler) HGDeleteUnusedUser(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSpace(r.URL.Query().Get("user_id"))
	actor, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePackage.FailTokenInvalid(w, r, "登录状态无效")
		return
	}
	if target == "" || len(target) > 255 {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.HGErrorResult{Code: HGResponsePackage.InvalidParam.Code, Message: "user_id 参数无效"})
		return
	}
	if target == actor {
		w.WriteHeader(http.StatusForbidden)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.HGErrorResult{Code: HGResponsePackage.Forbidden.Code, Message: "不能删除当前登录账号"})
		return
	}
	err := h.svc.HGDeleteUnusedUser(r.Context(), target, actor)
	if err != nil {
		status := http.StatusInternalServerError
		result := HGResponsePackage.HGErrorResult{Code: HGResponsePackage.InternalError.Code, Message: "删除用户失败，请稍后重试"}
		switch {
		case errors.Is(err, UserServicePackage.HGErrUserDeleteBusy):
			status = http.StatusTooManyRequests
			result = HGResponsePackage.HGErrorResult{Code: HGResponsePackage.RateLimit.Code, Message: err.Error()}
		case errors.Is(err, UserServicePackage.HGErrUserDeleteSelf):
			status = http.StatusForbidden
			result = HGResponsePackage.HGErrorResult{Code: HGResponsePackage.Forbidden.Code, Message: err.Error()}
		case UserServicePackage.HGUserDeleteIsNotFound(err):
			status = http.StatusNotFound
			result = HGResponsePackage.HGErrorResult{Code: HGResponsePackage.UserNotFound.Code, Message: "用户不存在"}
		case errors.Is(err, userrepository.HGErrUserHasDependencies), errors.Is(err, UserServicePackage.HGErrUserDeletionInProgress):
			status = http.StatusConflict
			result = HGResponsePackage.HGErrorResult{Code: HGResponsePackage.InvalidParam.Code, Message: err.Error()}
		}
		w.WriteHeader(status)
		HGResponsePackage.FailResult[string](w, r, result)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]string{"user_id": target})
}

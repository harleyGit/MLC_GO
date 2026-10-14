package handler

import (
	"MLC_GO/internal/modules/recharge/model"
	"MLC_GO/internal/modules/recharge/service"
	hgcontext "MLC_GO/internal/pkg/hg_context"
	response "MLC_GO/internal/response"
	"errors"
	"net/http"
	"net/url"
)

// HGHandler 是当前 JWT 用户的充值记录 HTTP 边界。
type HGHandler struct{ hgService *service.HGService } // 独立充值记录服务。

// HGNewHandler 注入服务，不执行外部 I/O。
func HGNewHandler(hgService *service.HGService) *HGHandler { return &HGHandler{hgService: hgService} }

// HGRecords 只信任 JWT 身份，拒绝重复或损坏的分页参数。
func (hgHandler *HGHandler) HGRecords(w http.ResponseWriter, r *http.Request) {
	hgUser, hgOK := hgcontext.CurrentUserID(r)
	if !hgOK {
		w.WriteHeader(http.StatusUnauthorized)
		response.FailTokenInvalid(w, r, "请先登录")
		return
	}
	hgQuery, hgErr := url.ParseQuery(r.URL.RawQuery)
	if hgErr != nil || len(hgQuery["cursor"]) > 1 || len(hgQuery["pageSize"]) > 1 {
		hgError(w, r, model.HGErrInvalid)
		return
	}
	hgPage, hgErr := hgHandler.hgService.HGRecords(r.Context(), hgUser, hgQuery.Get("cursor"), hgQuery.Get("pageSize"))
	if hgErr != nil {
		hgError(w, r, hgErr)
		return
	}
	response.SuccessResult(w, r, hgPage)
}

func hgError(w http.ResponseWriter, r *http.Request, hgErr error) {
	hgStatus, hgCode, hgMessage := http.StatusInternalServerError, response.InternalError.Code, "充值记录服务暂不可用"
	if errors.Is(hgErr, model.HGErrInvalid) {
		hgStatus, hgCode, hgMessage = http.StatusBadRequest, response.InvalidParam.Code, model.HGErrInvalid.Error()
	}
	w.WriteHeader(hgStatus)
	response.FailResult[string](w, r, response.HGErrorResult{Code: hgCode, Message: hgMessage})
}
